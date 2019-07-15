## s3cli
#### 1. Download prebuild binary
https://github.com/vager/s3cli/releases

#### 4. Configuration
Edit ~/.aws/credentials
```
[default]
aws_access_key_id=AK
aws_secret_access_key=SK

[ecs]
aws_access_key_id=AK
aws_secret_access_key=SK
```

## Usage
```
./s3cli -h
s3cli client tool for S3 Bucket/Object operation

Usage:
  s3cli [command]

Available Commands:
  acl          acl Bucket or Object
  createBucket create Bucket
  delete       delete Bucket or Object(s)
  deleteBucket delete bucket
  download     download Object
  getacl       get Bucket/Object acl
  head         head Bucket/Object
  help         Help about any command
  list         list Buckets or Objects
  listBuckets  list Buckets
  mpu          mpu Object
  presign      presign Object
  upload       upload Object

Flags:
  -d, --debug             print debug log
  -e, --endpoint string   endpoint (default "http://s3test.myshare.io:9090")
  -h, --help              help for s3cli
  -p, --profile string    profile in credential file
  -R, --region string     region (default "cn-north-1")
  -v, --version           print version

Use "s3cli [command] --help" for more information about a command.
```

## Example
##### Create(make) Bucket
```
./s3cli -e http://192.168.55.2:9020 -p ecs cb bucket1
```

##### List Buckets
```
./s3cli -e http://192.168.55.2:9020 -p ecs lb
bucket1"
```
or  
```
export S3CLI_ENDPOINT=http://192.168.55.2:9020
./s3cli -p ecs lb
bucket1
```

##### Upload file
```
./s3cli -e http://192.168.55.2:9020 -p ecs cb bucket1
./s3cli -e http://192.168.55.2:9020 -p ecs up bucket1 /etc/hosts
upload /etc/hosts to bucket1/hosts success
./s3cli -e http://192.168.55.2:9020 -p ecs up bucket1 /etc/resolv.conf -k key2
upload /etc/resolv.conf to bucket1/key2 success
```

##### Download file
```
./s3cli -e http://192.168.55.2:9020 -p ecs down bucket1 hosts
download hosts to hosts
./s3cli -e http://192.168.55.2:9020 -p ecs down bucket1 key2 resolv.conf
download key2 to resolv.conf
```

##### Presign get Object
```
./s3cli -e http://192.168.55.2:9020 -p ecs psg bucket1 hosts
```

##### Presign put Object 
```
./s3cli -e http://192.168.55.2:9020 -p ecs psg bucket1 host --put
```

##### List Objects
```
./s3cli -e http://192.168.55.2:9020 -p ecs ls bucket1
host
hosts
key1
key2
key3
```

##### List Objects with specified prefix
```
./s3cli -e http://192.168.55.2:9020 -p ecs ls bucket1 -x ke
key1
key2
key3
```

##### Delete Objects with specified prefix
```
./s3cli -e http://192.168.55.2:9020 -p ecs delete bucket1 key -x
3 Objects deleted
all 3 Objects deleted
```

##### Delete Bucket and all Objects
```
./s3cli -e http://192.168.55.2:9020 -p ecs delete bucket1
2 Objects deleted
Bucket bucket1 and 2 Objects deleted
```