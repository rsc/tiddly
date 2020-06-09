## Tiddly Git Backup

This is a simple Google App Engine app that will backup the Tiddly datastore to
a git repo.

The intention of this system is to have a safe automatic backup of your
TiddlyWiki and also to enable automated generation of [static
sites][static].

[static]: https://tiddlywiki.com/static/Generating%2520Static%2520Sites%2520with%2520TiddlyWiki.html

### Configuration

Copy `app.yaml.example` to `app.yaml` and configure the environment variables.

- `GITHTTP_USERNAME` usually your github username
- `GITHTTP_PASSWORD` usually a github personal access token secret key with `repo` access
- `GITHTTP_URL` usually a HTTPS URL to a github repo

### Deploy

Deploy this service into the same Google Cloud project you deploy tiddly into.

```
gcloud --project YOUR_PROJECT app deploy
```

And deploy the cron job:

```
gcloud app deploy cron.yaml
```
