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
	"sync"
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

var gd struct {
	repo git.Repository
	wt   *git.Worktree
	auth transport.AuthMethod
}

var mux sync.Mutex

func gitClone(url string, dir string) error {
	fmt.Printf("git clone %s %s --recursive\n", url, dir)
	r, err := git.PlainClone(dir, false, &git.CloneOptions{
		Auth:              gd.auth,
		URL:               url,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	})
	if err != nil {
		return err
	}

	gd.repo = *r

	ref, err := gd.repo.Head()
	if err != nil {
		return err
	}

	_, err = gd.repo.CommitObject(ref.Hash())
	if err != nil {
		return err
	}

	w, err := gd.repo.Worktree()
	if err != nil {
		return err
	}

	gd.wt = w

	return nil
}

func gitCommit(ctx context.Context) error {
	err := gd.wt.AddGlob("tiddlers/*")
	if err != nil {
		return err
	}

	status, err := gd.wt.Status()
	if err != nil {
		return err
	}

	// Don't commit and push if nothing changed
	if status.IsClean() {
		return nil
	}

	co, err := gd.wt.Commit("updates", &git.CommitOptions{
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
	obj, err := gd.repo.CommitObject(co)
	if err != nil {
		return err
	}

	fmt.Println(obj)

	fmt.Printf("git push\n")
	// push using default options
	err = gd.repo.Push(&git.PushOptions{
		Auth: gd.auth,
	})
	if err != nil {
		return err
	}

	return nil
}

func main() {
	var err error
	gituser := os.Getenv("GITHTTP_USERNAME")
	gitpass := os.Getenv("GITHTTP_PASSWORD")
	giturl := os.Getenv("GITHTTP_URL")

	if giturl == "" || gituser == "" || gitpass == "" {
		fmt.Printf("environment variables GITHTTP_USERNAME and GITHTTP_PASSWORD must be set\n")
		os.Exit(1)
	}

	gd.auth = &githttp.BasicAuth{
		Username: gituser,
		Password: gitpass,
	}

	err = gitClone(giturl, prefix)
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

	// only one go routine editing git repo at once
	mux.Lock()
	defer mux.Unlock()

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

	gitCommit(ctx)

	fmt.Fprintf(w, "OK\n")
}
