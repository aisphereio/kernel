# objectstorex

`objectstorex` 是 Aisphere Kernel 的统一对象存储模块。基于 Minio(minio-go/v7),兼容所有 S3 协议的服务(AWS S3、Minio、DigitalOcean Spaces 等)。

## 快速上手

```go
import (
    "github.com/aisphereio/kernel/objectstorex"
    _ "github.com/aisphereio/kernel/objectstorex/minio"
)

store, _ := objectstorex.New(objectstorex.Config{
    Driver:       "minio",
    Endpoint:     "localhost:9000",
    UseSSL:       false,
    AccessKey:    "minioadmin",
    SecretKey:    "minioadmin",
    Bucket:       "aihub",
    EnsureBucket: true, // 启动时自动建桶
})
defer store.Close()

// 上传
store.PutObject(ctx, "skills/demo/package.zip", reader, size, objectstorex.PutOptions{
    ContentType: "application/zip",
    Metadata:    map[string]string{"skill-name": "demo"},
})

// 下载
rc, info, _ := store.GetObject(ctx, "skills/demo/package.zip", objectstorex.GetOptions{})
defer rc.Close()

// 预签名 URL(给前端直传)
url, _ := store.PresignPut(ctx, "skills/demo/upload.zip", 15*time.Minute)

// 删除
store.DeleteObject(ctx, "skills/demo/package.zip")
```

## API

| 类别 | 方法 |
|---|---|
| Bucket 管理 | Bucket / BucketExists / EnsureBucket |
| CRUD | PutObject / GetObject / DeleteObject / StatObject |
| 列表 | ListObjects (prefix 过滤) |
| 复制 | CopyObject |
| 预签名 | PresignPut / PresignGet |
| 分片上传 | InitMultipartUpload / UploadPart / CompleteMultipartUpload / AbortMultipartUpload |
| 生命周期 | Close |

## 测试

```bash
# 单元测试(fake 内存实现,不需要 Docker)
go test ./objectstorex/... -short

# 集成测试(testcontainers,需要 Docker)
go test ./objectstorex/...

# 或用外部 Minio
KERNEL_OBJECTSTOREX_MINIO_ENDPOINT=localhost:9000 go test ./objectstorex/... -run TestIntegration
```
