package git

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net"
)

// See: https://confluence.atlassian.com/bitbucket/manage-webhooks-735643732.html
var bitbucketIPBlocks = []string{
	"131.103.20.160/27",
	"165.254.145.0/26",
	"104.192.143.0/24",
}

type BitbucketHook struct {}

type bbPush struct {
	Push struct {
			 Changes []struct {
				 New struct {
						 Name   string  `json:"name,omitempty"`
					 } `json:"new,omitempty"`
			 } `json:"changes,omitempty"`
		 } `json:"push,omitempty"`
}

func (b BitbucketHook) DoesHandle(h http.Header) bool {
	event := h.Get("X-Event-Key")

	// for Gitlab you can only use X-Gitlab-Event header to test if you could handle the request
	if event != "" {
		return true
	}
	return false
}

func (b BitbucketHook) Handle(w http.ResponseWriter, r *http.Request, repo *Repo) (int, error) {
//	if !b.verifyBitbucketIP(r.RemoteAddr) {
//		return http.StatusForbidden, errors.New("the request doesn't come from a valid IP")
//	}

	if r.Method != "POST" {
		return http.StatusMethodNotAllowed, errors.New("the request had an invalid method.")
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return http.StatusRequestTimeout, errors.New("could not read body from request")
	}

	event := r.Header.Get("X-Event-Key")
	if event == "" {
		return http.StatusBadRequest, errors.New("the 'X-Event-Key' header is required but was missing.")
	}

	switch event {
	case "repo:push":
		err := b.handlePush(body, repo)
		if err != nil {
			return http.StatusBadRequest, err
		}
	default:
		// return 400 if we do not handle the event type.
		return http.StatusBadRequest, nil
	}

	return http.StatusOK, nil
}


func (b BitbucketHook) handlePush(body []byte, repo *Repo) error {
	var push bbPush

	err := json.Unmarshal(body, &push)
	if err != nil {
		return err
	}

	if len(push.Push.Changes) == 0 {
		return errors.New("the push was incomplete, missing change list")
	}

    change := push.Push.Changes[0]
	if len(change.New.Name) == 0 {
		return errors.New("the push didn't contain a valid branch name")
	}

	branch := change.New.Name
	if branch == repo.Branch {
		Logger().Print("Received pull notification for the tracking branch, updating...\n")
		repo.Pull()
	}

	return nil
}

func (b BitbucketHook) verifyBitbucketIP(remoteIP string) bool {
	ipAddress := net.ParseIP(remoteIP)
	for _, cidr := range bitbucketIPBlocks {
		_, cidrnet, err := net.ParseCIDR(cidr)
		if err != nil {
			Logger().Printf("Error parsing CIDR block [%s]. Skipping...\n", cidr)
			continue
		}

		if cidrnet.Contains(ipAddress) {
			return true
		}
	}
	return false
}

/*
{
  "actor": User,
  "repository": Repository,
  "push": {
	"changes": [
	  {
		"new": {
		  "type": "branch",
		  "name": "name-of-branch",
		  "target": {
			"type": "commit",
			"hash": "709d658dc5b6d6afcd46049c2f332ee3f515a67d",
			"author": User,
			"message": "new commit message\n",
			"date": "2015-06-09T03:34:49+00:00",
			"parents": [
			  {
				"type": "commit",
				"hash": "1e65c05c1d5171631d92438a13901ca7dae9618c",
				"links": {
				  "self": {
					"href": "https://api.bitbucket.org/2.0/repositories/user_name/repo_name/commit/8cbbd65829c7ad834a97841e0defc965718036a0"
				  },
				  "html": {
					"href": "https://bitbucket.org/user_name/repo_name/commits/8cbbd65829c7ad834a97841e0defc965718036a0"
				  }
				}
			  }
			],
			"links": {
			  "self": {
				"href": "https://api.bitbucket.org/2.0/repositories/user_name/repo_name/commit/c4b2b7914156a878aa7c9da452a09fb50c2091f2"
			  },
			  "html": {
				"href": "https://bitbucket.org/user_name/repo_name/commits/c4b2b7914156a878aa7c9da452a09fb50c2091f2"
			  }
			},
		  },
		  "links": {
			"self": {
			  "href": "https://api.bitbucket.org/2.0/repositories/user_name/repo_name/refs/branches/master"
			},
			"commits": {
			  "href": "https://api.bitbucket.org/2.0/repositories/user_name/repo_name/commits/master"
			},
			"html": {
			  "href": "https://bitbucket.org/user_name/repo_name/branch/master"
			}
		  }
		},
		"old": {
		  "type": "branch",
		  "name": "name-of-branch",
		  "target": {
			"type": "commit",
			"hash": "1e65c05c1d5171631d92438a13901ca7dae9618c",
			"author": User,
			"message": "old commit message\n",
			"date": "2015-06-08T21:34:56+00:00",
			"parents": [
			  {
				"type": "commit",
				"hash": "e0d0c2041e09746be5ce4b55067d5a8e3098c843",
				"links": {
				  "self": {
					"href": "https://api.bitbucket.org/2.0/repositories/user_name/repo_name/commit/9c4a3452da3bc4f37af5a6bb9c784246f44406f7"
				  },
				  "html": {
					"href": "https://bitbucket.org/user_name/repo_name/commits/9c4a3452da3bc4f37af5a6bb9c784246f44406f7"
				  }
				}
			  }
			],
			"links": {
			  "self": {
				"href": "https://api.bitbucket.org/2.0/repositories/user_name/repo_name/commit/b99ea6dad8f416e57c5ca78c1ccef590600d841b"
			  },
			  "html": {
				"href": "https://bitbucket.org/user_name/repo_name/commits/b99ea6dad8f416e57c5ca78c1ccef590600d841b"
			  }
			}
		  },
		  "links": {
			"self": {
			  "href": "https://api.bitbucket.org/2.0/repositories/user_name/repo_name/refs/branches/master"
			},
			"commits": {
			  "href": "https://api.bitbucket.org/2.0/repositories/user_name/repo_name/commits/master"
			},
			"html": {
			  "href": "https://bitbucket.org/user_name/repo_name/branch/master"
			}
		  }
		},
		"links": {
		  "html": {
			"href": "https://bitbucket.org/user_name/repo_name/branches/compare/c4b2b7914156a878aa7c9da452a09fb50c2091f2..b99ea6dad8f416e57c5ca78c1ccef590600d841b"
		  },
		  "diff": {
			"href": "https://api.bitbucket.org/2.0/repositories/user_name/repo_name/diff/c4b2b7914156a878aa7c9da452a09fb50c2091f2..b99ea6dad8f416e57c5ca78c1ccef590600d841b"
		  },
		  "commits": {
			"href": "https://api.bitbucket.org/2.0/repositories/user_name/repo_name/commits?include=c4b2b7914156a878aa7c9da452a09fb50c2091f2&exclude=b99ea6dad8f416e57c5ca78c1ccef590600d841b"
		  }
		},
		"created": false,
		"forced": false,
		"closed": false,
		"commits": [
		  {
			"hash": "03f4a7270240708834de475bcf21532d6134777e",
			"type": "commit",
			"message": "commit message\n",
			"author": User,
			"links": {
			  "self": {
				"href": "https://api.bitbucket.org/2.0/repositories/user/repo/commit/03f4a7270240708834de475bcf21532d6134777e"
			  },
			  "html": {
				"href": "https://bitbucket.org/user/repo/commits/03f4a7270240708834de475bcf21532d6134777e"
			  }
			}
		  }
		],
		"truncated": false
	  }
	]
  }
}
 */