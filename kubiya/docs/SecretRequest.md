# SecretRequest

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Secrets** | Pointer to [**[]SecretType**](SecretType.md) |  | [optional] 

## Methods

### NewSecretRequest

`func NewSecretRequest() *SecretRequest`

NewSecretRequest instantiates a new SecretRequest object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewSecretRequestWithDefaults

`func NewSecretRequestWithDefaults() *SecretRequest`

NewSecretRequestWithDefaults instantiates a new SecretRequest object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetSecrets

`func (o *SecretRequest) GetSecrets() []SecretType`

GetSecrets returns the Secrets field if non-nil, zero value otherwise.

### GetSecretsOk

`func (o *SecretRequest) GetSecretsOk() (*[]SecretType, bool)`

GetSecretsOk returns a tuple with the Secrets field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSecrets

`func (o *SecretRequest) SetSecrets(v []SecretType)`

SetSecrets sets Secrets field to given value.

### HasSecrets

`func (o *SecretRequest) HasSecrets() bool`

HasSecrets returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


