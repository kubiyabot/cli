# \SecretsAPI

All URIs are relative to *https://api.kubiya.com*

Method | HTTP request | Description
------------- | ------------- | -------------
[**V1SecretsGet**](SecretsAPI.md#V1SecretsGet) | **Get** /v1/secrets | Get secrets
[**V1SecretsPost**](SecretsAPI.md#V1SecretsPost) | **Post** /v1/secrets | Create secrets



## V1SecretsGet

> map[string]string V1SecretsGet(ctx).Execute()

Get secrets

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
	resp, r, err := apiClient.SecretsAPI.V1SecretsGet(context.Background()).Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error when calling `SecretsAPI.V1SecretsGet``: %v\n", err)
		fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
	}
	// response from `V1SecretsGet`: map[string]string
	fmt.Fprintf(os.Stdout, "Response from `SecretsAPI.V1SecretsGet`: %v\n", resp)
}
```

### Path Parameters

This endpoint does not need any parameter.

### Other Parameters

Other parameters are passed through a pointer to a apiV1SecretsGetRequest struct via the builder pattern


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


## V1SecretsPost

> []WorkflowResponse V1SecretsPost(ctx).Body(body).Execute()

Create secrets

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
	body := *openapiclient.NewSecretRequest() // SecretRequest | 

	configuration := openapiclient.NewConfiguration()
	apiClient := openapiclient.NewAPIClient(configuration)
	resp, r, err := apiClient.SecretsAPI.V1SecretsPost(context.Background()).Body(body).Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error when calling `SecretsAPI.V1SecretsPost``: %v\n", err)
		fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
	}
	// response from `V1SecretsPost`: []WorkflowResponse
	fmt.Fprintf(os.Stdout, "Response from `SecretsAPI.V1SecretsPost`: %v\n", resp)
}
```

### Path Parameters



### Other Parameters

Other parameters are passed through a pointer to a apiV1SecretsPostRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**SecretRequest**](SecretRequest.md) |  | 

### Return type

[**[]WorkflowResponse**](WorkflowResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

