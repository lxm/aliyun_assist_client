package taskengine

import (
	"encoding/json"
	"fmt"
	neturl "net/url"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/aliyun/aliyun_assist_client/agent/log"
	"github.com/aliyun/aliyun_assist_client/agent/util"
	"github.com/aliyun/aliyun_assist_client/agent/util/timetool"
)

type FetchReason string
const (
	FetchOnKickoff FetchReason = "kickoff"
	FetchOnStartup FetchReason = "startup"
)

type taskInfo struct {
	TaskInfo   RunTaskInfo `json:"task"`
	OutputInfo OutputInfo  `json:"output"`
	Repeat     RunTaskRepeatType `json:"repeat"`
}

type sendFileInfo struct {
	TaskInfo   SendFileTaskInfo `json:"task"`
	OutputInfo OutputInfo       `json:"output"`
}

type tasks struct {
	RunTasks      []taskInfo     `json:"run"`
	StopTasks     []taskInfo     `json:"stop"`
	TestTasks     []taskInfo     `json:"test"`
	SendFileTasks []sendFileInfo `json:"file"`
	SessionTasks      []SessionTaskInfo     `json:"session"`
	InstanceId    string         `json:"instanceId"`
}

type taskCollection struct {
	runInfos []RunTaskInfo
	stopInfos []RunTaskInfo
	testInfos []RunTaskInfo
	sendFiles []SendFileTaskInfo
	sessionInfos []SessionTaskInfo
}

func newTaskCollection() *taskCollection {
	taskInfos := taskCollection{
		runInfos: []RunTaskInfo{},
		stopInfos: []RunTaskInfo{},
		testInfos: []RunTaskInfo{},
		sendFiles: []SendFileTaskInfo{},
		sessionInfos: []SessionTaskInfo{},
	}
	return &taskInfos
}

func parseTaskInfo(jsonStr string) *taskCollection {
	logger := log.GetLogger().WithFields(logrus.Fields{
		"module": "parseTaskInfo",
	})

	taskInfos := newTaskCollection()

	var task_lists tasks
	err := json.Unmarshal([]byte(jsonStr), &task_lists)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"jsonString": jsonStr,
		}).WithError(err).Errorln("Invalid task info json")
		return taskInfos
	}

	for _, v := range task_lists.RunTasks {
		runTaskInfo, err := v.toRunTaskInfo(task_lists.InstanceId)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"runTask": v,
			}).WithError(err).Errorln("Invalid run task info")
			continue
		}
		taskInfos.runInfos = append(taskInfos.runInfos, runTaskInfo)
	}

	for _, stopTask := range task_lists.StopTasks {
		stopTaskInfo, err := stopTask.toRunTaskInfo(task_lists.InstanceId)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"stopTask": stopTask,
			}).WithError(err).Errorln("Invalid stop task info")
			continue
		}
		taskInfos.stopInfos = append(taskInfos.stopInfos, stopTaskInfo)
	}
	for _, testTask := range task_lists.TestTasks {
		testTaskInfo, err := testTask.toRunTaskInfo(task_lists.InstanceId)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"testTask": testTask,
			}).WithError(err).Errorln("Invalid test task info")
			continue
		}
		taskInfos.testInfos = append(taskInfos.testInfos, testTaskInfo)
	}
	for _, sendFileTask := range task_lists.SendFileTasks {
		sendFile := sendFileTask.TaskInfo
		sendFile.Output = sendFileTask.OutputInfo
		taskInfos.sendFiles = append(taskInfos.sendFiles, sendFile)
	}

	for _, sessionTask := range task_lists.SessionTasks {
		taskInfos.sessionInfos = append(taskInfos.sessionInfos, sessionTask)
	}

	return taskInfos
}

func FetchTaskList(reason FetchReason, taskId string, taskType int, isColdstart bool) *taskCollection {
	if util.GetServerHost() == "" {
		return newTaskCollection()
	}

	url := util.GetFetchTaskListService()
	switch reason {
	case FetchOnKickoff:
		url = url + "?reason=" + string(reason)
	case FetchOnStartup:
		url = url + fmt.Sprintf("?reason=%s&cold_start=%t", reason, isColdstart)
	default:
		log.GetLogger().WithFields(logrus.Fields{
			"reason": reason,
		}).Errorln("Invalid reason for fetching tasks")
		return newTaskCollection()
	}
	if taskType == SessionTaskType {
		url = util.GetFetchSessionTaskListService()
		if taskId != "" {
			url = url + "?channelId=" + taskId
		}
	} else {
		if taskId != "" {
			url = url + "&taskId=" + taskId
		}
		// Append Unix timestamp and timezone name of current wall clock
		currentTime, currentOffsetFromUTC, timezoneName := timetool.NowWithTimezoneName()
		escapedTimezoneName := neturl.QueryEscape(timezoneName)
		url += fmt.Sprintf("&currentTime=%d&offset=%d&timeZone=%s", timetool.ToAccurateTime(currentTime), currentOffsetFromUTC, escapedTimezoneName)
	}

	var err error
	var response string
	response, err = util.HttpPost(url, "", "")

	for i := 0; i < 3 && err != nil; i++ {
		time.Sleep(time.Duration(2) * time.Second)
		response, err = util.HttpPost(url, "", "")
	}

	if err != nil {
		return newTaskCollection()
	}

	taskInfos := parseTaskInfo(response)
	return taskInfos
}

func (t *taskInfo) toRunTaskInfo(instanceId string) (RunTaskInfo, error) {
	runTaskInfo := t.TaskInfo
	runTaskInfo.InstanceId = instanceId
	runTaskInfo.Output = t.OutputInfo
	runTaskInfo.Repeat = t.Repeat

	// Compatible with no `Repeat` field in task info pulled
	// TODO: Remove compatibility code when `Repeat` field fully available
	if runTaskInfo.Repeat == "" {
		if runTaskInfo.Cronat != "" {
			runTaskInfo.Repeat = RunTaskCron
		} else {
			runTaskInfo.Repeat = RunTaskOnce
		}
	}

	// Prepare values of environment parameters if enableParameter is true
	if runTaskInfo.EnableParameter {
		runTaskInfo.EnvironmentArguments = map[string]string{
			"InstanceId": instanceId,
			"CommandId": runTaskInfo.CommandId,
			"InvokeId": runTaskInfo.TaskId,
		}
	}

	return runTaskInfo, nil
}
