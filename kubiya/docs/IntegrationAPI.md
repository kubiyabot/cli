# \IntegrationAPI

All URIs are relative to *https://api.kubiya.com*

Method | HTTP request | Description
------------- | ------------- | -------------
[**V1IntegrationsGet**](IntegrationAPI.md#V1IntegrationsGet) | **Get** /v1/integrations | Get integrations
[**V1IntegrationsVendorDelete**](IntegrationAPI.md#V1IntegrationsVendorDelete) | **Delete** /v1/integrations/{vendor} | Delete vendor integration
[**V1IntegrationsVendorGet**](IntegrationAPI.md#V1IntegrationsVendorGet) | **Get** /v1/integrations/{vendor} | Get vendor integration
[**V1IntegrationsVendorStatusPost**](IntegrationAPI.md#V1IntegrationsVendorStatusPost) | **Post** /v1/integrations/{vendor}/status | Update vendor integration status



## V1IntegrationsGet

> map[string]string V1IntegrationsGet(ctx).Execute()

Get integrations

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
	resp, r, err := apiClient.IntegrationAPI.V1IntegrationsGet(context.Background()).Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error when calling `IntegrationAPI.V1IntegrationsGet``: %v\n", err)
		fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
	}
	// response from `V1IntegrationsGet`: map[string]string
	fmt.Fprintf(os.Stdout, "Response from `IntegrationAPI.V1IntegrationsGet`: %v\n", resp)
}
```

### Path Parameters

This endpoint does not need any parameter.

### Other Parameters

Other parameters are passed through a pointer to a apiV1IntegrationsGetRequest struct via the builder pattern


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


## V1IntegrationsVendorDelete

> map[string]string V1IntegrationsVendorDelete(ctx, vendor).Execute()

Delete vendor integration

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
	vendor := "vendor_example" // string | Vendor name

	configuration := openapiclient.NewConfiguration()
	apiClient := openapiclient.NewAPIClient(configuration)
	resp, r, err := apiClient.IntegrationAPI.V1IntegrationsVendorDelete(context.Background(), vendor).Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error when calling `IntegrationAPI.V1IntegrationsVendorDelete``: %v\n", err)
		fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
	}
	// response from `V1IntegrationsVendorDelete`: map[string]string
	fmt.Fprintf(os.Stdout, "Response from `IntegrationAPI.V1IntegrationsVendorDelete`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**vendor** | **string** | Vendor name | 

### Other Parameters

Other parameters are passed through a pointer to a apiV1IntegrationsVendorDeleteRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


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


## V1IntegrationsVendorGet

> map[string]string V1IntegrationsVendorGet(ctx, vendor).Execute()

Get vendor integration

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
	vendor := "vendor_example" // string | Vendor name

	configuration := openapiclient.NewConfiguration()
	apiClient := openapiclient.NewAPIClient(configuration)
	resp, r, err := apiClient.IntegrationAPI.V1IntegrationsVendorGet(context.Background(), vendor).Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error when calling `IntegrationAPI.V1IntegrationsVendorGet``: %v\n", err)
		fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
	}
	// response from `V1IntegrationsVendorGet`: map[string]string
	fmt.Fprintf(os.Stdout, "Response from `IntegrationAPI.V1IntegrationsVendorGet`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**vendor** | **string** | Vendor name | 

### Other Parameters

Other parameters are passed through a pointer to a apiV1IntegrationsVendorGetRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


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


## V1IntegrationsVendorStatusPost

> map[string]string V1IntegrationsVendorStatusPost(ctx, vendor).Body(body).Execute()

Update vendor integration status

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
	vendor := "vendor_example" // string | Vendor name
	body := *openapiclient.NewV1IntegrationsVendorStatusPostRequest() // V1IntegrationsVendorStatusPostRequest | Status update

	configuration := openapiclient.NewConfiguration()
	apiClient := openapiclient.NewAPIClient(configuration)
	resp, r, err := apiClient.IntegrationAPI.V1IntegrationsVendorStatusPost(context.Background(), vendor).Body(body).Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error when calling `IntegrationAPI.V1IntegrationsVendorStatusPost``: %v\n", err)
		fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
	}
	// response from `V1IntegrationsVendorStatusPost`: map[string]string
	fmt.Fprintf(os.Stdout, "Response from `IntegrationAPI.V1IntegrationsVendorStatusPost`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**vendor** | **string** | Vendor name | 

### Other Parameters

Other parameters are passed through a pointer to a apiV1IntegrationsVendorStatusPostRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **body** | [**V1IntegrationsVendorStatusPostRequest**](V1IntegrationsVendorStatusPostRequest.md) | Status update | 

### Return type

**map[string]string**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

