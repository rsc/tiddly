package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
)

const prefix = "/tmp/gitbackup"

type Tiddler struct {
	Rev  int      `datastore:"Rev,noindex"`
	Meta string   `datastore:"Meta,noindex"`
	Text string   `datastore:"Text,noindex"`
	Tags []string `datastore:"Tags,noindex"`
}

type GitDir struct {
	repo git.Repository
	wt   *git.Worktree
	auth transport.AuthMethod
}

func NewGitDir(url string, auth transport.AuthMethod, dir string) (*GitDir, error) {
	gc := GitDir{
		auth: auth,
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Printf("git clone %s %s --recursive\n", url, dir)
		r, err := git.PlainClone(dir, false, &git.CloneOptions{
			Auth:              gc.auth,
			URL:               url,
			RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		})
		if err != nil {
			return nil, err
		}

		gc.repo = *r
	} else {
		r, err := git.PlainOpen(dir)
		if err != nil {
			return nil, err
		}
		gc.repo = *r

		w, err := r.Worktree()
		if err != nil {
			return nil, err
		}

		err = w.Pull(&git.PullOptions{RemoteName: "origin"})
		if err != nil {
			return nil, err
		}
	}

	ref, err := gc.repo.Head()
	if err != nil {
		return nil, err
	}

	_, err = gc.repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}

	w, err := gc.repo.Worktree()
	if err != nil {
		return nil, err
	}

	gc.wt = w

	return &gc, nil
}

func (g GitDir) Commit(ctx context.Context) error {
	err := g.wt.AddGlob("tiddlers/*")
	if err != nil {
		return err
	}

	status, err := g.wt.Status()
	if err != nil {
		return err
	}

	// Don't commit and push if nothing changed
	if status.IsClean() {
		return nil
	}

	co, err := g.wt.Commit("updates", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "TiddlyWiki Git Backup",
			Email: "none@example.com",
			When:  time.Now(),
		},
	})

	if err != nil {
		return err
	}

	fmt.Println(status)
	obj, err := g.repo.CommitObject(co)
	if err != nil {
		return err
	}

	fmt.Println(obj)

	fmt.Printf("git push\n")
	// push using default options
	err = g.repo.Push(&git.PushOptions{
		Auth: g.auth,
	})
	if err != nil {
		return err
	}

	return nil
}

var gd *GitDir

func main() {
	var err error
	gituser := os.Getenv("GITHTTP_USERNAME")
	gitpass := os.Getenv("GITHTTP_PASSWORD")
	giturl := os.Getenv("GITHTTP_URL")

	if giturl == "" || gituser == "" || gitpass == "" {
		fmt.Printf("environment variables GITHTTP_USERNAME and GITHTTP_PASSWORD must be set\n")
		os.Exit(1)
	}

	auth := githttp.BasicAuth{
		Username: gituser,
		Password: gitpass,
	}

	gd, err = NewGitDir(giturl, &auth, prefix)
	if err != nil {
		panic(err)
	}

	http.HandleFunc("/", index)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	appengine.Main()

	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}

}

func index(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "bad method", 405)
		return
	}

	ctx := appengine.NewContext(r)
	q := datastore.NewQuery("Tiddler")
	// Only need Meta, but get no results if we do this.
	if false {
		q = q.Project("Meta")
	}
	it := q.Run(ctx)

	dir := filepath.Join(prefix, "tiddlers")
	err := gd.wt.RemoveGlob("tiddlers/*")
	if err != nil {
		println("ERR", err.Error())
		http.Error(w, err.Error(), 500)
		return
	}
	os.RemoveAll(dir)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		println("ERR", err.Error())
		http.Error(w, err.Error(), 500)
		return
	}

	for {
		var t Tiddler
		_, err := it.Next(&t)
		if err != nil {
			if err == datastore.Done {
				break
			}
			println("ERR", err.Error())
			http.Error(w, err.Error(), 500)
			return
		}
		if len(t.Meta) == 0 {
			continue
		}

		var buf bytes.Buffer
		var js map[string]interface{}

		err = json.Unmarshal([]byte(t.Meta), &js)
		if err != nil {
			println("ERR cannot unmarshal")
			continue
		}

		// Sort keys to ensure file stability for git
		mk := make([]string, 0, len(js))
		for k := range js {
			mk = append(mk, k)
		}
		sort.Strings(mk)

		for _, k := range mk {
			if k == "tags" {
				tags := js[k].([]interface{})
				if len(tags) == 0 {
					continue
				}
				var t string
				sep := ""
				for _, v := range tags {
					t = t + sep + v.(string)
					sep = " "
				}
				js[k] = t
			}
			buf.Write([]byte(fmt.Sprintf("%s: %v\n", k, js[k])))
		}

		buf.Write([]byte("\n"))

		buf.Write([]byte(t.Text))
		err = ioutil.WriteFile(filepath.Join(dir, fmt.Sprintf("%v.tid", js["title"])), buf.Bytes(), 0644)
	}

	gd.Commit(ctx)

	fmt.Fprintf(w, "OK\n", r.URL.Path)
}
