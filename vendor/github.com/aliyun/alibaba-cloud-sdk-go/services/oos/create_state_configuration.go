package oos

//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
// Code generated by Alibaba Cloud SDK Code Generator.
// Changes may cause incorrect behavior and will be lost if the code is regenerated.

import (
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/responses"
)

// CreateStateConfiguration invokes the oos.CreateStateConfiguration API synchronously
func (client *Client) CreateStateConfiguration(request *CreateStateConfigurationRequest) (response *CreateStateConfigurationResponse, err error) {
	response = CreateCreateStateConfigurationResponse()
	err = client.DoAction(request, response)
	return
}

// CreateStateConfigurationWithChan invokes the oos.CreateStateConfiguration API asynchronously
func (client *Client) CreateStateConfigurationWithChan(request *CreateStateConfigurationRequest) (<-chan *CreateStateConfigurationResponse, <-chan error) {
	responseChan := make(chan *CreateStateConfigurationResponse, 1)
	errChan := make(chan error, 1)
	err := client.AddAsyncTask(func() {
		defer close(responseChan)
		defer close(errChan)
		response, err := client.CreateStateConfiguration(request)
		if err != nil {
			errChan <- err
		} else {
			responseChan <- response
		}
	})
	if err != nil {
		errChan <- err
		close(responseChan)
		close(errChan)
	}
	return responseChan, errChan
}

// CreateStateConfigurationWithCallback invokes the oos.CreateStateConfiguration API asynchronously
func (client *Client) CreateStateConfigurationWithCallback(request *CreateStateConfigurationRequest, callback func(response *CreateStateConfigurationResponse, err error)) <-chan int {
	result := make(chan int, 1)
	err := client.AddAsyncTask(func() {
		var response *CreateStateConfigurationResponse
		var err error
		defer close(result)
		response, err = client.CreateStateConfiguration(request)
		callback(response, err)
		result <- 1
	})
	if err != nil {
		defer close(result)
		callback(nil, err)
		result <- 0
	}
	return result
}

// CreateStateConfigurationRequest is the request struct for api CreateStateConfiguration
type CreateStateConfigurationRequest struct {
	*requests.RpcRequest
	ScheduleType       string `position:"Query" name:"ScheduleType"`
	ClientToken        string `position:"Query" name:"ClientToken"`
	Description        string `position:"Query" name:"Description"`
	Targets            string `position:"Query" name:"Targets"`
	TemplateVersion    string `position:"Query" name:"TemplateVersion"`
	ScheduleExpression string `position:"Query" name:"ScheduleExpression"`
	TemplateName       string `position:"Query" name:"TemplateName"`
	ConfigureMode      string `position:"Query" name:"ConfigureMode"`
	Tags               string `position:"Query" name:"Tags"`
	Parameters         string `position:"Query" name:"Parameters"`
}

// CreateStateConfigurationResponse is the response struct for api CreateStateConfiguration
type CreateStateConfigurationResponse struct {
	*responses.BaseResponse
	RequestId          string             `json:"RequestId" xml:"RequestId"`
	StateConfiguration StateConfiguration `json:"StateConfiguration" xml:"StateConfiguration"`
}

// CreateCreateStateConfigurationRequest creates a request to invoke CreateStateConfiguration API
func CreateCreateStateConfigurationRequest() (request *CreateStateConfigurationRequest) {
	request = &CreateStateConfigurationRequest{
		RpcRequest: &requests.RpcRequest{},
	}
	request.InitWithApiInfo("oos", "2019-06-01", "CreateStateConfiguration", "", "")
	request.Method = requests.POST
	return
}

// CreateCreateStateConfigurationResponse creates a response to parse from CreateStateConfiguration response
func CreateCreateStateConfigurationResponse() (response *CreateStateConfigurationResponse) {
	response = &CreateStateConfigurationResponse{
		BaseResponse: &responses.BaseResponse{},
	}
	return
}
