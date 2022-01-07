package pluginmanager

import (
	"io"
	"time"
	"os"
	"os/exec"
	"unsafe"
	"errors"
	"golang.org/x/sys/windows"
	"github.com/aliyun/aliyun_assist_client/agent/util/process"
	"github.com/aliyun/aliyun_assist_client/agent/log"
	"github.com/sirupsen/logrus"
)

// We use this struct to retreive process handle(which is unexported)
// from os.Process using unsafe operation.
type processHandle struct {
	Pid    int
	Handle uintptr
}

type waitProcessResult struct {
	processState *os.ProcessState
	err error
}

type ProcessExitGroup windows.Handle

func NewProcessExitGroup() (ProcessExitGroup, error) {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(
		handle,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info))); err != nil {
		return 0, err
	}
	return ProcessExitGroup(handle), nil
}

func (g ProcessExitGroup) Dispose() error {
	log.GetLogger().Errorf("command done, child processes will be kill by terminate the job object")
	return windows.CloseHandle(windows.Handle(g))
}

func (g ProcessExitGroup) AddProcess(p *os.Process) error {
	return windows.AssignProcessToJobObject(
		windows.Handle(g),
		windows.Handle((*processHandle)(unsafe.Pointer(p)).Handle))
}

func syncRunKillGroup(workingDir string, commandName string, commandArguments []string, stdoutWriter io.Writer, stderrWriter io.Writer,
	timeOut int) (exitCode int, status int, err error) {
	g, err := NewProcessExitGroup()
	if err != nil {
		return 1, process.Fail, err
	}
	defer func() {
		g.Dispose()
	}()

	cmd := exec.Command(commandName, commandArguments...)
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter
	cmd.Dir = workingDir

	if err = cmd.Start(); err != nil {
		exitCode = 1
		return exitCode, process.Fail, err
	}

	if err = g.AddProcess(cmd.Process); err != nil {
		return 1, process.Fail, err
	}

	finished := make(chan waitProcessResult, 1)
	go func() {
		processState, err := cmd.Process.Wait()
		finished <- waitProcessResult{
			processState: processState,
			err: err,
		}
	}()

	select {
	case waitProcessResult := <-finished:
		if waitProcessResult.processState != nil {
			if waitProcessResult.err != nil {
				log.GetLogger().WithFields(logrus.Fields{
					"processState": waitProcessResult.processState,
				}).WithError(waitProcessResult.err).Error("os.Process.Wait() returns error with valid process state")
			}
			exitCode = waitProcessResult.processState.ExitCode()
			// Sleep 200ms to allow remaining data to be copied back
			time.Sleep(time.Duration(200) * time.Millisecond)
			// Explicitly break select statement in case timer also times out
			break
		} else {
			exitCode = 1
			return exitCode, process.Fail, waitProcessResult.err
		}
	case <-time.After(time.Duration(timeOut) * time.Second):
		log.GetLogger().Errorln("Timeout in run command.", commandName)
		exitCode = 1
		status = process.Timeout
		err = errors.New("timeout")
	}
	return exitCode, status, err
}