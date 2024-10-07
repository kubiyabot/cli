# HealthCheckResponse

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Error** | Pointer to **string** |  | [optional] 
**Health** | Pointer to **string** |  | [optional] 

## Methods

### NewHealthCheckResponse

`func NewHealthCheckResponse() *HealthCheckResponse`

NewHealthCheckResponse instantiates a new HealthCheckResponse object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewHealthCheckResponseWithDefaults

`func NewHealthCheckResponseWithDefaults() *HealthCheckResponse`

NewHealthCheckResponseWithDefaults instantiates a new HealthCheckResponse object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetError

`func (o *HealthCheckResponse) GetError() string`

GetError returns the Error field if non-nil, zero value otherwise.

### GetErrorOk

`func (o *HealthCheckResponse) GetErrorOk() (*string, bool)`

GetErrorOk returns a tuple with the Error field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetError

`func (o *HealthCheckResponse) SetError(v string)`

SetError sets Error field to given value.

### HasError

`func (o *HealthCheckResponse) HasError() bool`

HasError returns a boolean if a field has been set.

### GetHealth

`func (o *HealthCheckResponse) GetHealth() string`

GetHealth returns the Health field if non-nil, zero value otherwise.

### GetHealthOk

`func (o *HealthCheckResponse) GetHealthOk() (*string, bool)`

GetHealthOk returns a tuple with the Health field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetHealth

`func (o *HealthCheckResponse) SetHealth(v string)`

SetHealth sets Health field to given value.

### HasHealth

`func (o *HealthCheckResponse) HasHealth() bool`

HasHealth returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


