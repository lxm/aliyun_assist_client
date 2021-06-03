package taskengine

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	heavylock "github.com/viney-shih/go-lock"

	"github.com/aliyun/aliyun_assist_client/agent/log"
	"github.com/aliyun/aliyun_assist_client/agent/taskengine/timermanager"
	"github.com/aliyun/aliyun_assist_client/agent/util/atomicutil"
)

const (
	ErrUpdatingProcedureRunning = -7
)

const (
	NormalTaskType = 0
	SessionTaskType = 1
)

// PeriodicTaskSchedule consists of timer and reusable invocation data structure
// for periodic task
type PeriodicTaskSchedule struct {
	timer              *timermanager.Timer
	reusableInvocation *Task
}

var (
	// FetchingTaskLock indicates whether one goroutine is fetching tasks
	FetchingTaskLock heavylock.CASMutex
	// FetchingTaskCounter indicates how many goroutines are fetching tasks
	FetchingTaskCounter atomicutil.AtomicInt32

	// Indicating whether is enabled to fetch tasks, ONLY operated by atomic operation
	_neverDirectWrite_Atomic_FetchingTaskEnabled int32 = 0

	_periodicTaskSchedules     map[string]*PeriodicTaskSchedule
	_periodicTaskSchedulesLock sync.Mutex
)

func init() {
	FetchingTaskLock = heavylock.NewCASMutex()

	_periodicTaskSchedules = make(map[string]*PeriodicTaskSchedule)
}

// EnableFetchingTask sets prviate indicator to allow fetching tasks
func EnableFetchingTask() {
	atomic.StoreInt32(&_neverDirectWrite_Atomic_FetchingTaskEnabled, 1)
}

func isEnabledFetchingTask() bool {
	state := atomic.LoadInt32(&_neverDirectWrite_Atomic_FetchingTaskEnabled)
	return state != 0
}

func Fetch(from_kick bool, taskId string, taskType int, isColdstart bool) int {
	// Fetching task should be allowed before all core components of agent have
	// been correctly initialized. This critical indicator would be set at the
	// end of program.run method
	if !isEnabledFetchingTask() {
		log.GetLogger().WithFields(logrus.Fields{
			"from_kick": from_kick,
		}).Infoln("Fetching tasks is disabled due to network is not ready")
		return 0
	}

	// NOTE: sync.Mutex from Go standard library does not support try-lock
	// operation like std::mutex in C++ STL, which makes it slightly hard for
	// goroutines of fetching tasks and checking updates to coopearate gracefully.
	// Futhermore, it does not support try-lock operation with specified timeout,
	// which makes it hard for goroutines of fetching tasks to wait in queue but
	// just throw many message about lock accquisition failure confusing others.
	// THUS heavy weight lock from github.com/viney-shih/go-lock library is used
	// to provide graceful locking mechanism for goroutine coopeartion. The cost
	// would be, some performance lost.
	if !FetchingTaskLock.TryLockWithTimeout(time.Duration(2) * time.Second) {
		log.GetLogger().WithFields(logrus.Fields{
			"from_kick": from_kick,
		}).Infoln("Fetching tasks is canceled due to another running fetching or updating process.")
		return ErrUpdatingProcedureRunning
	}
	// Immediately release fetchingTaskLock to let other goroutine fetching
	// tasks go, but keep updating safe
	FetchingTaskLock.Unlock()

	// Increase fetchingTaskCounter to indicate there is a goroutine fetching
	// tasks, which the updating goroutine MUST notice and decrease it to let
	// updating goroutine go.
	FetchingTaskCounter.Add(1)
	defer FetchingTaskCounter.Add(-1)

	var task_size int
	if from_kick {
		task_size = fetchTasks(FetchOnKickoff, taskId, taskType,false)
	} else {
		task_size = fetchTasks(FetchOnStartup, taskId, taskType, isColdstart)
	}

	for i := 0; i < 1 && from_kick && task_size == 0; i++ {
		time.Sleep(time.Duration(3) * time.Second)
		task_size = fetchTasks(FetchOnKickoff, taskId, taskType, false)
	}

	return task_size
}

func fetchTasks(reason FetchReason, taskId string, taskType int, isColdstart bool) int {
	taskInfos := FetchTaskList(reason, taskId, taskType, isColdstart)
	SendFiles(taskInfos.sendFiles)
	DoSessionTask(taskInfos.sessionInfos)
	for _, v := range taskInfos.runInfos {
		dispatchRunTask(v)
	}

	for _, v := range taskInfos.stopInfos {
		dispatchStopTask(v)
	}

	for _, v := range taskInfos.testInfos {
		dispatchTestTask(v)
	}

	return len(taskInfos.runInfos) + len(taskInfos.stopInfos) + len(taskInfos.sessionInfos) + len(taskInfos.sendFiles)
}

func dispatchRunTask(taskInfo RunTaskInfo) {
	fetchLogger := log.GetLogger().WithFields(logrus.Fields{
		"TaskId": taskInfo.TaskId,
		"Phase":  "Fetched",
	})
	fetchLogger.Info("Fetched to be run")

	taskFactory := GetTaskFactory()
	// Tasks should not be duplicately handled
	if taskFactory.ContainsTaskByName(taskInfo.TaskId) {
		fetchLogger.Warning("Ignored duplicately fetched task")
		return
	}

	// Reuse specified logger across task scheduling phase
	scheduleLogger := log.GetLogger().WithFields(logrus.Fields{
		"TaskId": taskInfo.TaskId,
		"Phase":  "Scheduling",
	})
	switch taskInfo.Repeat {
	case RunTaskOnce, RunTaskNextRebootOnly, RunTaskEveryReboot:
		t := NewTask(taskInfo)

		scheduleLogger.Info("Schedule non-periodic task")
		// Non-periodic tasks are managed by TaskFactory
		taskFactory.AddTask(t)
		pool := GetPool()
		pool.RunTask(func ()  {
			t.Run()
			taskFactory := GetTaskFactory()
			taskFactory.RemoveTaskByName(t.taskInfo.TaskId)
		})
		scheduleLogger.Info("Scheduled for pending or running")
	case RunTaskPeriod:
		// Periodic tasks are managed by _periodicTaskSchedules
		err := schedulePeriodicTask(taskInfo)
		if err != nil {
			scheduleLogger.WithFields(logrus.Fields{
				"taskInfo": taskInfo,
			}).WithError(err).Errorln("Failed to schedule periodic task")
		} else {
			scheduleLogger.Infoln("Succeed to schedule periodic task")
		}
	default:
		scheduleLogger.WithFields(logrus.Fields{
			"taskInfo": taskInfo,
		}).Errorln("Unknown repeat type")
	}
}

func dispatchStopTask(taskInfo RunTaskInfo) {
	log.GetLogger().WithFields(logrus.Fields{
		"TaskId": taskInfo.TaskId,
		"Phase":  "Fetched",
	}).Info("Fetched to be canceled")

	cancelLogger := log.GetLogger().WithFields(logrus.Fields{
		"TaskId": taskInfo.TaskId,
		"Phase":  "Cancelling",
	})
	taskFactory := GetTaskFactory()
	switch taskInfo.Repeat {
	case RunTaskOnce, RunTaskNextRebootOnly, RunTaskEveryReboot:
		// NOTE: Non-periodic tasks are managed by TaskFactory. Those tasks
		// does not exist in TaskFactory need not to be canceled.
		if !taskFactory.ContainsTaskByName(taskInfo.TaskId) {
			cancelLogger.Warning("Ignore task not found due to finished or error")
			return
		}

		cancelLogger.Info("Cancel task and invocation")
		scheduledTask, _ := taskFactory.GetTask(taskInfo.TaskId)
		scheduledTask.Cancel()
		cancelLogger.Info("Canceled task and invocation")
	case RunTaskPeriod:
		// Periodic tasks are managed by _periodicTaskSchedules
		err := cancelPeriodicTask(taskInfo)
		if err != nil {
			cancelLogger.WithFields(logrus.Fields{
				"taskInfo": taskInfo,
			}).WithError(err).Errorln("Failed to cancel periodic task")
		} else {
			cancelLogger.Infoln("Succeed to cancel periodic task")
		}
	default:
		cancelLogger.WithFields(logrus.Fields{
			"taskInfo": taskInfo,
		}).Errorln("Unknown repeat type")
	}
}

func dispatchTestTask(taskInfo RunTaskInfo) {
	fetchLogger := log.GetLogger().WithFields(logrus.Fields{
		"TaskId": taskInfo.TaskId,
		"Phase":  "Fetched",
	})
	fetchLogger.Info("Fetched to be run")

	taskFactory := GetTaskFactory()
	// Tasks should not be duplicately handled
	if taskFactory.ContainsTaskByName(taskInfo.TaskId) {
		fetchLogger.Warning("Ignored duplicately fetched task")
		return
	}

	// Reuse specified logger across task scheduling phase
	scheduleLogger := log.GetLogger().WithFields(logrus.Fields{
		"TaskId": taskInfo.TaskId,
		"Phase":  "Scheduling",
	})
	switch taskInfo.Repeat {
	case RunTaskOnce, RunTaskPeriod, RunTaskNextRebootOnly, RunTaskEveryReboot:
		t := NewTask(taskInfo)

		scheduleLogger.Info("Schedule testing task to be pre-checked")
		pool := GetPrecheckPool()
		pool.RunTask(func () {
			t.PreCheck(true)
		})
		scheduleLogger.Info("Scheduled testing task to be pre-checked")
	default:
		scheduleLogger.WithFields(logrus.Fields{
			"taskInfo": taskInfo,
		}).Errorln("Unknown repeat type")
	}
}

func (s *PeriodicTaskSchedule) startExclusiveInvocation() {
	// Reuse specified logger across task scheduling phase
	invocateLogger := log.GetLogger().WithFields(logrus.Fields{
		"TaskId": s.reusableInvocation.taskInfo.TaskId,
		"Phase":  "PeriodicInvocating",
	})

	// NOTE: TaskPool has been closely wired with TaskFactory, thus:
	taskFactory := GetTaskFactory()
	// (3) Existed invocation in TaskFactory means task is running.
	if taskFactory.ContainsTaskByName(s.reusableInvocation.taskInfo.TaskId) {
		invocateLogger.Warn("Skip invocation since overlapped with existing invocation")
		return
	}

	invocateLogger.Info("Schedule new invocation of periodic task")
	// (2) Every time of invocation need to add itself into TaskFactory at first.
	taskFactory.AddTask(s.reusableInvocation)
	pool := GetPool()
	pool.RunTask(func ()  {
		s.reusableInvocation.Run()
		taskFactory := GetTaskFactory()
		taskFactory.RemoveTaskByName(s.reusableInvocation.taskInfo.TaskId)
	})
	invocateLogger.Info("Scheduled new pending or running invocation")
}

func schedulePeriodicTask(taskInfo RunTaskInfo) error {
	timerManager := timermanager.GetTimerManager()
	if timerManager == nil {
		return errors.New("Global TimerManager instance is not initialized")
	}

	_periodicTaskSchedulesLock.Lock()
	defer _periodicTaskSchedulesLock.Unlock()

	// Reuse specified logger across task scheduling phase
	scheduleLogger := log.GetLogger().WithFields(logrus.Fields{
		"TaskId": taskInfo.TaskId,
		"Phase":  "Scheduling",
	})

	// 1. Check whether periodic task has been registered in local task storage,
	// and had corresponding timer in timer manager
	_, ok := _periodicTaskSchedules[taskInfo.TaskId]
	if ok {
		scheduleLogger.Warn("Ignore periodic task registered in local")
		return nil
	}

	// 2. Create PeriodicTaskSchedule object
	scheduleLogger.Info("Create timer of periodic task")
	periodicTaskSchedule := &PeriodicTaskSchedule{
		timer: nil,
		// Invocations of periodic task is not allowed to overlap, so Task struct
		// for invocation data can be reused.
		reusableInvocation: NewTask(taskInfo),
	}
	// 3. Create cron expression timer and register into TimerManager
	// NOTE: reusableInvocation is binded to callback via closure feature of golang,
	// maybe explicit passing into callback like "data" for traditional thread
	// would be better
	timer, err := timerManager.CreateCronTimer(func() {
		periodicTaskSchedule.startExclusiveInvocation()
	}, taskInfo.Cronat)
	if err != nil {
		return err
	}
	// then bind it to periodicTaskSchedule object
	periodicTaskSchedule.timer = timer
	scheduleLogger.Info("Created timer of periodic task")

	// 4. Register schedule object into _periodicTaskSchedules
	_periodicTaskSchedules[taskInfo.TaskId] = periodicTaskSchedule
	scheduleLogger.Info("Registered periodic task")

	// 5. Current API of TimerManager requires manual startup of timer
	scheduleLogger.Info("Run timer of periodic task")
	_, err = timer.Run()
	if err != nil {
		timerManager.DeleteTimer(periodicTaskSchedule.timer)
		delete(_periodicTaskSchedules, taskInfo.TaskId)
		return err
	}
	scheduleLogger.Info("Running timer of periodic task")

	return nil
}

func cancelPeriodicTask(taskInfo RunTaskInfo) error {
	timerManager := timermanager.GetTimerManager()
	if timerManager == nil {
		return errors.New("Global TimerManager instance is not initialized")
	}

	_periodicTaskSchedulesLock.Lock()
	defer _periodicTaskSchedulesLock.Unlock()

	cancelLogger := log.GetLogger().WithFields(logrus.Fields{
		"TaskId": taskInfo.TaskId,
		"Phase":  "Cancelling",
	})

	// 1. Check whether task is registered in local storage
	periodicTaskSchedule, ok := _periodicTaskSchedules[taskInfo.TaskId]
	if !ok {
		return fmt.Errorf("Unregistered periodic task %s", taskInfo.TaskId)
	}

	// 2. Delete timer of periodic task from TimerManager, which contains stopping
	// timer operation
	timerManager.DeleteTimer(periodicTaskSchedule.timer)
	cancelLogger.Infof("Stop and remove timer of periodic task")

	// 3. Delete registered task record from local storage
	delete(_periodicTaskSchedules, taskInfo.TaskId)
	cancelLogger.Infof("Deregistered periodic task")

	// 4. Cancel existing invocation of periodic task and send ACK
	runningInvocation, ok := GetTaskFactory().GetTask(taskInfo.TaskId)
	if ok {
		cancelLogger.Infof("Cancel running invocation of periodic task")
		runningInvocation.Cancel()
		cancelLogger.Infof("Canceled running invocation of periodic task")
	} else {
		cancelLogger.Infof("Not need to cancel running invocation of periodic task")
		// Since no running
		lastInvocation := periodicTaskSchedule.reusableInvocation
		lastInvocation.sendOutput("canceled", lastInvocation.getReportString(lastInvocation.output))
		cancelLogger.Infof("Sent canceled ACK with output of last invocation")
	}
	return nil
}
