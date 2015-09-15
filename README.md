# git

Middleware for [Caddy](https://caddyserver.com).

[![Build Status](https://img.shields.io/travis/mholt/caddy.svg?style=flat-square)](https://travis-ci.org/abiosoft/caddy-git)

git clones a git repository into the site. This makes it possible to deploy your site with a simple git push.

The git directive does not chain in a handler. Instead, it starts a service routine that runs during the lifetime of the server. When the server starts, it clones the repository. While the server is still up, it pulls the latest every so often. In regular git fashion, a download only includes changes so it is very efficient.

If a pull fails, the service will retry up to three times. If the pull was not successful by then, it won't try again until the next interval.

**Requirements**: This directive requires git to be installed. Also, private repositories may only be accessed from Linux or Mac systems. (Contributions are welcome that make private repositories work on Windows.)

### Syntax

```
git repo [path]
```
* **repo** is the URL to the repository; SSH and HTTPS URLs are supported
* **path** is the path, relative to site root, to clone the repository into; default is site root

This simplified syntax pulls from master every 3600 seconds (1 hour) and only works for public repositories.

For more control or to use a private repository, use the following syntax:

```
git [repo path] {
	repo     repo
    path     path
	branch   branch
	key      key
	interval interval
	hook     path secret
	then     command [args...]
}
```
* **repo** is the URL to the repository; SSH and HTTPS URLs are supported.
* **path** is the path, relative to site root, to clone the repository into; default is site root.
* **branch** is the branch or tag to pull; default is master branch. **`{latest}`** is a placeholder for latest tag which ensures the most recent tag is always pulled.
* **key** is the path to the SSH private key; only required for private repositories.
* **interval** is the number of seconds between pulls; default is 3600 (1 hour), minimum 5.
* **path** and **secret** are used to create a webhook which pulls the latest right after a push. This is currently limited to **Github**, **Gitlab** and **BitBucket**.
* **command** is a command to execute after successful pull; followed by **args** which are any arguments to pass to the command.

Each property in the block is optional. The path and repo may be specified on the first line, as in the first syntax, or they may be specified in the block with other values.

### Examples

Public repository pulled into site root every hour:
```
git github.com/user/myproject
```

Public repository pulled into the "subfolder" directory in the site root:
```
git github.com/user/myproject /subfolder
```

Private repository pulled into the "subfolder" directory with tag v1.0 once per day:
```
git {
	repo     git@github.com:user/myproject
	branch   v1.0
	key      /home/user/.ssh/id_rsa
	path     subfolder
	interval 86400
}
```

Generate a static site with [Hugo](http://gohugo.io) after each pull:
```
git github.com/user/site {
	path  ../
	then  hugo --destination=/home/user/hugosite/public
}
```

Part of a Caddyfile for a PHP site that gets changes from a private repo:
```
git git@github.com:user/myphpsite {
	key /home/user/.ssh/id_rsa
}
fastcgi / 127.0.0.1:9000 php
```

