// Copyright (c) 2017-2018 Alibaba Group Holding Limited.
#include "gtest/gtest.h"

#include <string>
#include <algorithm>

#include "utils/http_request.h"
#include "utils/AssistPath.h"
#include "utils/Log.h"
#include "utils/FileUtil.h"
#include "curl/curl.h"
#include "utils/CheckNet.h"
#include "utils/OsVersion.h"
#include "utils/singleton.h"
#include "jsoncpp/json.h"
#include "schedule_task.h"
#include "plugin/timer_manager.h"
#include "./task.h"
#include "./task_factory.h"
#include "./fetch_task.h"

void init_log() {
  AssistPath path_service("");
  std::string log_path = path_service.GetLogPath();
  log_path += FileUtils::separator();
  log_path += "aliyun_assist_test.log";
  Log::Initialise(log_path);
}

TEST(TestTaskEgine, AddRemoveTask) {
  task_engine::TaskInfo info;
  info.command_id = "RunBatScript";
  info.task_id = "t-001";
  task_engine::Task* task =
      Singleton<task_engine::TaskFactory>::I().CreateTask(info);
  if(!task) {
    EXPECT_EQ(true, false);
  }
  task_engine::Task* task1 =
      Singleton<task_engine::TaskFactory>::I().GetTask(info.task_id);
  EXPECT_EQ(task1->GetTaskInfo().task_id, "t-001");

  Singleton<task_engine::TaskFactory>::I().RemoveTask(info.task_id);
  task_engine::Task* task2 =
      Singleton<task_engine::TaskFactory>::I().GetTask(info.task_id);
  EXPECT_EQ(task2, nullptr);
}

TEST(TestTaskEgine, FetchTask) {
  std::vector<task_engine::TaskInfo> task_info;
  std::string mocked_str("[{\"taskInstanceID\":\"i-4743a05f-fc6a-469b-82c1-0ee3fd3b15f4\",\"taskItem\":{\"TaskID\":\"t-4743a05f-fc6a-469b-82c1-0ee3fd3b15f4\"}}]");
  task_engine::TaskFetch fetch;
  fetch.TestFetchTasks(mocked_str, task_info);
  std::string value("i-4743a05f-fc6a-469b-82c1-0ee3fd3b15f4");
  int t = value.compare(task_info[0].instance_id);
  EXPECT_EQ(0, value.compare(task_info[0].instance_id));
}

#if defined(_WIN32)
TEST(TestTaskEgine, RunBatScript) {
  init_log();
  Log::Info("begin test");
  task_engine::TaskInfo info;
  info.command_id = "RunBatScript";
  info.task_id = "t-120bf664f8454a7cbb64b0841c87f474";
  info.content = "echo test";
  info.time_out = "3600";
  task_engine::Task* task =
      Singleton<task_engine::TaskSchedule>::I().Schedule(info);
  Sleep(2000);
  bool finished = false;
  if(task->GetOutput().find("test") != std::string::npos) {
    finished = true;
  }
  EXPECT_EQ(true, finished);
}

TEST(TestTaskEgine, RunPowshellScript) {
  task_engine::TaskInfo info;
  info.command_id = "RunPowserShellScript";
  info.task_id = "t-120bf664f8454a7cbb64b0841c87f475";
  info.content = "echo test";
  info.time_out = "3600";
  task_engine::Task* task =
    Singleton<task_engine::TaskSchedule>::I().Schedule(info);
  Sleep(4000);
  bool finished = false;
  if (task->GetOutput().find("test") != std::string::npos) {
    finished = true;
  }
  EXPECT_EQ(true, finished);
}

TEST(TestTaskEgine, RunPeriodTask) {
  task_engine::TaskInfo info;
  info.command_id = "RunPowserShellScript";
  info.task_id = "t-120bf664f8454a7cbb64b0841c87f476";
  info.content = "echo test";
  info.time_out = "3600";
  info.cronat = "*/1 * * * * *";
  Singleton<task_engine::TimerManager>::I().Start();
  task_engine::Task* task =
    Singleton<task_engine::TaskSchedule>::I().Schedule(info);
  Sleep(5*1000);
  // Todo() watch the log to check the task status.
}
#else
TEST(TestTaskEgine, RunShellScript) {
  init_log();
  Log::Info("begin test");
  task_engine::TaskInfo info;
  info.command_id = "RunShellScript";
  info.task_id = "t-120bf664f8454a7cbb64b0841c87f474";
  info.content = "echo test";
  info.time_out = "3600";
  task_engine::Task* task =
      Singleton<task_engine::TaskSchedule>::I().Schedule(info);
  sleep(3);
  bool finished = false;
  if (task->GetOutput().find("test") != std::string::npos) {
    finished = true;
  }
  EXPECT_EQ(true, finished);
}
#endif



