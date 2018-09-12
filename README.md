# git

Middleware for [Caddy](https://caddyserver.com).

[![Build Status](https://travis-ci.org/abiosoft/caddy-git.svg?branch=master)](https://travis-ci.org/abiosoft/caddy-git)

git clones a git repository into the site. This makes it possible to deploy your site with a simple git push.

The git directive starts a service routine that runs during the lifetime of the server. When the service starts, it clones the repository. While the server is still up, it pulls the latest every so often. You can also set up a webhook to pull immediately after a push. In regular git fashion, a pull only includes changes, so it is very efficient.

If a pull fails, the service will retry up to three times. If the pull was not successful by then, it won't try again until the next interval.

**Requirements:** This directive requires git to be installed. Also, private repositories may only be accessed from Linux or Mac systems. (Contributions are welcome that make private repositories work on Windows.)

## Syntax

```
git repo [path]
```
* **repo** is the URL to the repository; SSH and HTTPS URLs are supported
* **path** is the path, relative to site root, to clone the repository into; default is site root

This simplified syntax pulls from master every 3600 seconds (1 hour) and only works for public repositories.

For more control or to use a private repository, use the following syntax:

```
git [repo path] {
	repo        repo
	path        path
	branch      branch
	key         key
	interval    interval
	clone_args  args
	pull_args   args
	hook        path secret
	hook_type   type
	then        command [args...]
	then_long   command [args...]
}
```
* **repo** is the URL to the repository; SSH and HTTPS URLs are supported.
* **path** is the path to clone the repository into; default is site root. It can be absolute or relative (to site root).
* **branch** is the branch or tag to pull; default is master branch. **`{latest}`** is a placeholder for latest tag which ensures the most recent tag is always pulled.
* **key** is the path to the SSH private key; only required for private repositories.
* **interval** is the number of seconds between pulls; default is 3600 (1 hour), minimum 5. An interval of -1 disables periodic pull.
* **clone_args** is the additional cli args to pass to `git clone` e.g. `--depth=1`. `git clone` is called when the source is being fetched the first time.
* **pull_args** is the additional cli args to pass to `git pull` e.g. `-s recursive -X theirs`. `git pull` is used when the source is being updated.
* **path** and **secret** are used to create a webhook which pulls the latest right after a push. This is limited to the [supported webhooks](#supported-webhooks). **secret** is currently supported for GitHub, Gitlab and Travis hooks only.
* **type** is webhook type to use. The webhook type is auto detected by default but it can be explicitly set to one of the [supported webhooks](#supported-webhooks). This is a requirement for generic webhook.
* **command** is a command to execute after successful pull; followed by **args** which are any arguments to pass to the command. You can have multiple lines of this for multiple commands. **then_long** is for long executing commands that should run in background.

Each property in the block is optional. The path and repo may be specified on the first line, as in the first syntax, or they may be specified in the block with other values.

### Webhooks

A webhook is an interface between a git repository and an external server. On Github, the simplest webhook makes a request to a 3rd-party URL when the repository is pushed to. You can set up a Github webhook at `github.com/[username]/[repository]/settings/hooks`, and a [Travis webhook](https://docs.travis-ci.com/user/notifications/#Configuring-webhook-notifications) in your `.travis.yml`. Make sure your webhooks are set to deliver JSON data!

The JSON payload should include [at least a `ref` key](#user-content-generic-format), but all the default supported webhooks will handle this for you.

The hook URL is the URL Caddy will watch for requests on; if your url is, for example `/__github_webhook__` and Caddy is hosting `https://example.com`, when a request is made to `https://example.com/__github_webhook__` Caddy will intercept this request and check that the secret in the request (configured wherever you configure your webhooks) and the secret in your Caddyfile match. If the request is valid, Caddy will `git pull` its local copy of the repo to update your site as soon as you push new data. It may be useful to then use a [post-merge](https://git-scm.com/docs/githooks#_post_merge) script or another git hook to rebuild any needed files (updating [SASS](http://sass-lang.com/) styles and regenerating [Hugo](https://gohugo.io/) sites are common use-cases), although the [`then`](#user-content-then-example) parameter can also be used for simpler cases.

Note that because the hook URL is used as an API endpoint, you shouldn't have any content / files at its corresponding location in your website.

#### Supported Webhooks

* [github](https://github.com)
* [gitlab](https://gitlab.com)
* [bitbucket](https://bitbucket.org)
* [travis](https://travis-ci.org)
* [gogs](https://gogs.io)
* [gitee](https://gitee.com)
* generic

## Examples

Public repository pulled into site root every hour:
```
git github.com/user/myproject
```

Public repository pulled into the "subfolder" directory in the site root:
```
git github.com/user/myproject subfolder
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

<a name="then-example"></a>
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

Specifying a webhook:
```
git git@github.com:user/site {
	hook /webhook secret-password
}
```

You might need quotes `"secret-password"` around your secret if it contains any special characters, or you get an error.

<a name="generic_format"></a>
Generic webhook payload: `<branch>` is branch name e.g. `master`.
```
{
	"ref" : "refs/heads/<branch>"
}
```

