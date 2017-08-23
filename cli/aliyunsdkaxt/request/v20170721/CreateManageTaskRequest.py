# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
#
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.

from aliyunsdkcore.request import RpcRequest
class CreateManageTaskRequest(RpcRequest):

	def __init__(self):
		RpcRequest.__init__(self, 'axt', '2017-07-21', 'CreateManageTask')

	def get_cron(self):
		return self.get_query_params().get('cron')

	def set_cron(self,cron):
		self.add_query_param('cron',cron)

	def get_instanceIdss(self):
		return self.get_query_params().get('instanceIdss')

	def set_instanceIdss(self,instanceIdss):
		for i in range(len(instanceIdss)):	
			self.add_query_param('instanceIds.' + bytes(i + 1) , instanceIdss[i]);

	def get_instMgCmdId(self):
		return self.get_query_params().get('instMgCmdId')

	def set_instMgCmdId(self,instMgCmdId):
		self.add_query_param('instMgCmdId',instMgCmdId)