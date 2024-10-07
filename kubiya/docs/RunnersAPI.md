# \RunnersAPI

All URIs are relative to *https://api.kubiya.com*

Method | HTTP request | Description
------------- | ------------- | -------------
[**V1RunnersGet**](RunnersAPI.md#V1RunnersGet) | **Get** /v1/runners | Get runners
[**V1RunnersRunnerHealthGet**](RunnersAPI.md#V1RunnersRunnerHealthGet) | **Get** /v1/runners/{runner}/health | Get runner health



## V1RunnersGet

> map[string]string V1RunnersGet(ctx).Execute()

Get runners

### Example

```go
package main

import (
	"context"
	"fmt"
	"os"
	openapiclient "github.com/GIT_USER_ID/GIT_REPO_ID"
)

func main() {

	configuration := openapiclient.NewConfiguration()
	apiClient := openapiclient.NewAPIClient(configuration)
	resp, r, err := apiClient.RunnersAPI.V1RunnersGet(context.Background()).Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error when calling `RunnersAPI.V1RunnersGet``: %v\n", err)
		fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
	}
	// response from `V1RunnersGet`: map[string]string
	fmt.Fprintf(os.Stdout, "Response from `RunnersAPI.V1RunnersGet`: %v\n", resp)
}
```

### Path Parameters

This endpoint does not need any parameter.

### Other Parameters

Other parameters are passed through a pointer to a apiV1RunnersGetRequest struct via the builder pattern


### Return type

**map[string]string**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## V1RunnersRunnerHealthGet

> HealthCheckResponse V1RunnersRunnerHealthGet(ctx, runner).Execute()

Get runner health

### Example

```go
package main

import (
	"context"
	"fmt"
	"os"
	openapiclient "github.com/GIT_USER_ID/GIT_REPO_ID"
)

func main() {
	runner := "runner_example" // string | Runner name

	configuration := openapiclient.NewConfiguration()
	apiClient := openapiclient.NewAPIClient(configuration)
	resp, r, err := apiClient.RunnersAPI.V1RunnersRunnerHealthGet(context.Background(), runner).Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error when calling `RunnersAPI.V1RunnersRunnerHealthGet``: %v\n", err)
		fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
	}
	// response from `V1RunnersRunnerHealthGet`: HealthCheckResponse
	fmt.Fprintf(os.Stdout, "Response from `RunnersAPI.V1RunnersRunnerHealthGet`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**runner** | **string** | Runner name | 

### Other Parameters

Other parameters are passed through a pointer to a apiV1RunnersRunnerHealthGetRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


### Return type

[**HealthCheckResponse**](HealthCheckResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

