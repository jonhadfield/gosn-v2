## about

####Note: This is an early release with significant changes. Please take a backup before using with any real data and report any issues you find.

This package is used to persist encrypted SN items to a local [key-value store](https://github.com/etcd-io/bbolt) and
then only sync deltas on subsequent calls.

## usage

_under construction_

### sign in
```go
sio, _ := gosn.SignIn(gosn.SignInInput{
    Email:    "user@example.com",
    Password: "topsecret",
})
```
### get cache session
```go
cs, _ := cache.ImportSession(&sio.Session, "")
```
### sync items to database
```go
cso, _ := cache.Sync(cache.SyncInput{
    Session: cs,
})
```

### retrieve items
```go
var cItems []cache.Item
_ = cso.DB.All(&cItems)
```
