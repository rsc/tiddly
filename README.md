# TiddlyWiki App Engine Server

This is a minimal Google App Engine app, written in Go, that can serve
as the back end for a personal [TiddlyWiki](http://tiddlywiki.com/)

The [TiddlyWiki5](https://github.com/Jermolene/TiddlyWiki5) implementation
has a number of back end options. This app implements the backend expected
by the “[TiddlyWeb](http://tiddlyweb.com/) and TiddlySpace components” plugin.

The usual way to deploy TiddlyWeb is to run a fairly complex Python web server
program. I'd rather not. Instead I implemented a minimal Go server that responds
appropriately to the (relatively few) needed JSON API calls.

## Authentication

The TiddlyWeb JSON API envisions a multiuser system in which different users have
access to different sets of tiddlers. This Go server contains none of that:
it assumes that all users have full access to everything, although it does record
who created which tiddlers.

Authentication is controlled by [Google IAP][iap] as a “belt and suspenders”
measure. When deploying the application you will need to enable and [configure
IAP][configure-iap] with the addresses you want to have access.

[iap]: https://cloud.google.com/go/getting-started/authenticate-users-with-iap
[configure-iap]: https://cloud.google.com/go/getting-started/authenticate-users-with-iap

## Data model

The app stores the current tiddlers in Cloud Datastore as Tiddler entities.
It also stores every version of every tiddler as TiddlerHistory entities.
Currently nothing reads the TiddlerHistory, but in case of a mistake that
wipes out important Tiddler contents it should be possible to reconstruct
lost data from the TiddlerHistory.

The TiddlyWiki downloaded as index.html that runs in the browser
downloads (through the JSON API) a master list of all tiddlers and their
metadata when the page first loads and then lazily fetches individual 
tiddler content on demand.

## Deployment

Create an Google App Engine standard app and deploy with

	gcloud --project=your-app app deploy

Then visit https://your-app.appspot.com/. As noted above, only admins
will have access to the content.

## Backup

There is an optional service called [`gitbackup`][gitbackup] that can backup
the TiddlyWiki datastore to git periodically.

[gitbackup]: https://github.com/philips/tiddly/tree/master/gitbackup

## Plugins

TiddlyWiki supports extension through plugins. 
Plugins need to be in the downloaded index.html, not lazily
like other tiddlers. Therefore, installing a plugin means 
updating a local copy of index.html and redeploying it to
the server.

## Macros

TiddlyWiki allows tiddlers with the tag `$:/tags/Macro` to contain
global macro definitions made available to all tiddlers.
The lazy loading of tiddler bodies interferes with this: something
has to load the tiddler body before the macros it contains take effect.
To work around this, the app includes the body of all macro tiddlers
in the initial tiddler list (which otherwise does not contain bodies).
This is sufficient to make macros take effect on reload.

For some reason, no such special hack is needed for `$:/tags/Stylesheet` tiddlers.

## Synchronization

If you set Control Panel > Info > Basics > Default tiddlers by clicking
“retain story ordering”, then the story list (the list of tiddlers shown on the page)
is written to the server as it changes and is polled back from the server every 60 seconds.
This means that if you have the web site open in two different browsers 
(for example, at home and at work), changes to what you're viewing in one
propagate to the other.

## TiddlyWiki base image

The TiddlyWiki code is stored in and served from index.html, which
(as you can see by clicking on the Tools tab) is TiddlyWiki version 5.1.21.

Plugins must be pre-baked into the TiddlyWiki file, not stored on the server
as lazily loaded Tiddlers. The index.html in this directory is 5.1.21 with
the TiddlyWeb and Markdown plugins added. The TiddlyWeb plugin is
required, so that index.html talks back to the server for content.

The process for preparing a new index.html is:

- Open tiddlywiki-5.1.21.html in your web browser.
- Click the control panel (gear) icon.
- Click the Plugins tab.
- Click "Get more plugins".
- Click "Open plugin library".
- Type "tiddlyweb" into the search box. The "TiddlyWeb and TiddlySpace components" should appear.
- Click Install. A bar at the top of the page should say "Please save and reload for the changes to take effect."
- Click the icon next to save, and an updated file will be downloaded.
- Open the downloaded file in the web browser.
- Repeat, adding any more plugins.
- Copy the final download to index.html.
