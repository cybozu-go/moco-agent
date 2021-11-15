# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [proto/agentrpc.proto](#proto/agentrpc.proto)
    - [CloneRequest](#moco.CloneRequest)
    - [CloneResponse](#moco.CloneResponse)
  
    - [Agent](#moco.Agent)
  
- [Scalar Value Types](#scalar-value-types)



<a name="proto/agentrpc.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## proto/agentrpc.proto



<a name="moco.CloneRequest"></a>

### CloneRequest
CloneRequest is the request message to invoke MySQL CLONE command.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| host | [string](#string) |  | host is the donor host in the own cluster |
| port | [int32](#int32) |  | port is the port number where the donor host |
| user | [string](#string) |  | user is the MySQL user who has BACKUP_ADMIN privilege in the donor host. |
| password | [string](#string) |  | password for the above user. |
| init_user | [string](#string) |  | localhost user to initialize cloned database for MOCO. |
| init_password | [string](#string) |  | password for init_user. |
| boot_timeout | [google.protobuf.Duration](#google.protobuf.Duration) |  | wait up to this duration for mysqld to boot after clone. |






<a name="moco.CloneResponse"></a>

### CloneResponse
CloneResponse is the response message of Clone.





 

 

 


<a name="moco.Agent"></a>

### Agent
Agent provides services for MOCO.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Clone | [CloneRequest](#moco.CloneRequest) | [CloneResponse](#moco.CloneResponse) | Clone invokes MySQL CLONE command initializes the cloned database for MOCO. It does _not_ start the replication (START REPLICA). Actually, it works as follows.

1. Configure `clone_donor_valid_list` global variable to allow the donor instance.

2. Invoke `CLONE INSTANCE` with `user` and `password` in the CloneRequest.

3. Initialize the database for MOCO using `init_user` and `init_password`.

For 2, the user must have BACKUP_ADMIN and REPLICATION SLAVE privilege. For 3, the init_user must have ALL privilege with GRANT OPTION. The init_user is used only via UNIX domain socket, so its host can be `localhost`.

The donor database should have prepared these two users beforehand. |

 



## Scalar Value Types

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <a name="double" /> double |  | double | double | float | float64 | double | float | Float |
| <a name="float" /> float |  | float | float | float | float32 | float | float | Float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum or Fixnum (as required) |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="bool" /> bool |  | bool | boolean | boolean | bool | bool | boolean | TrueClass/FalseClass |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode | string | string | string | String (UTF-8) |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str | []byte | ByteString | string | String (ASCII-8BIT) |

