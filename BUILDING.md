# Building from Source

Follow the instructions below to build from source. [Go](http://golang.org/doc/install) must be installed.

### 1. Install Caddydev

```
$ go get github.com/caddyserver/caddydev
```

### 2. Pull Git Add-on
```
$ go get github.com/abiosoft/caddy-git
```

### 3. Execute
```
$ cd $GOPATH/src/github.com/abiosoft/caddy-git
$ caddydev
```
## Other options

### Execute from another directory
Copy the bundled caddydev config over to the directory.
```
$ cp $GOPATH/src/github.com/abiosoft/caddy-git/config.json config.json
$ caddydev
```
Or pass path to `config.json` as flag to caddydev.
```
$ caddydev --conf $GOPATH/src/github.com/abiosoft/caddy-git/config.json
```

### Generate Binary
Generate a Caddy binary that includes Git add-on.  
```
$ cd $GOPATH/src/github.com/abiosoft/caddy-git
$ caddydev -o caddy
$ ./caddy
```

### Note
Caddydev is more suited to development purpose. To add other add-ons to Caddy alongside Git, download from [Caddy's download page](https://caddyserver.com/download) or use [Caddyext](https://github.com/caddyserver/caddyext).