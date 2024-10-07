# HttpError

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Code** | Pointer to **int32** |  | [optional] 
**Error** | Pointer to **string** |  | [optional] 

## Methods

### NewHttpError

`func NewHttpError() *HttpError`

NewHttpError instantiates a new HttpError object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewHttpErrorWithDefaults

`func NewHttpErrorWithDefaults() *HttpError`

NewHttpErrorWithDefaults instantiates a new HttpError object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetCode

`func (o *HttpError) GetCode() int32`

GetCode returns the Code field if non-nil, zero value otherwise.

### GetCodeOk

`func (o *HttpError) GetCodeOk() (*int32, bool)`

GetCodeOk returns a tuple with the Code field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCode

`func (o *HttpError) SetCode(v int32)`

SetCode sets Code field to given value.

### HasCode

`func (o *HttpError) HasCode() bool`

HasCode returns a boolean if a field has been set.

### GetError

`func (o *HttpError) GetError() string`

GetError returns the Error field if non-nil, zero value otherwise.

### GetErrorOk

`func (o *HttpError) GetErrorOk() (*string, bool)`

GetErrorOk returns a tuple with the Error field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetError

`func (o *HttpError) SetError(v string)`

SetError sets Error field to given value.

### HasError

`func (o *HttpError) HasError() bool`

HasError returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


