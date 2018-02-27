# Grafana Dashboards Manager

The Grafana Dashboards Manager is a tool to help you manage your Grafana dashboards using Git.

## Manager

The manager is split in two parts:

### The puller

The puller is a tool that will pull all the dashboards from the Grafana API, except the ones with a name starting with a specific prefix (if provided in the configuration file), and commit them to the Git repository if needed (and push them to the remote afterwards).

To determine if a dashboard sould be commited to the repository, the puller relies on Grafana's dashboard version management. It will store the versions of all known dashboards (in a file called `versions.json`, which it will create if it doesn't exist), and commit changes to a dashboard only if the version retrieved from the Grafana API has a greater version number than the one stored in `versions.json` (if none is stored, it will systematically commit the retrieved dashboard).

If a dashboard has changes to be commited, its JSON description will be stored in a JSON file at the root of the repository (named `[dashboard slug].json`), and will be added to the Git index. Once all new or modified files have been added to the index, the puller creates a commit with the detail of the update in the commit message, then pushes it to the remote.

If there wasn't any error causing it to `panic`, the puller exits once all commited changes have been pushed to the Git remote.

**If you don't wish to version your dashboards via Git** but rather to just back them up on your disk, you can do so using the "simple sync" mode. More info about it can be found in the comments from `config.example.yaml`, under the `git` settings example.

### The pusher

The pusher is a tool that will watch a repository and relay any changes made to it to the Grafana instance. It works in two modes:

* `webhook`, which exposes a expose a webhook to a given address (`interface:port/path`) and process incoming push events sent by the Git server
* `git-pull`, which pulls the `master` branch from the Git remote at a given frequency and checks if any file has been added, modified or removed from the repository

For every push event on the `master` branch of the repository (in `webhook` mode), or every commit on this same branch (in `git-pull` mode), it will look at the files added or modified by the pushed commits (ignoring the ones with a name starting with a specific prefix (if provided in the configuration file)). It will then proceed to push them to the Grafana API to update modified dashboards or create added ones.

It will then call the puller to have all the files up to date. This is mainly done to update the version number of each dashboard, as Grafana updates them automatically when a new or updated dashboard is pushed.

Please note that, in its default mode, the pusher will only push new or modified dashboards to the Grafana API. If the file for a dashboard is removed from the Git repository, the dashboard won't be deleted on the Grafana instance, unless specifically asked (see below for more details).

Please also note that, because of how deeply intricated with Git it is, the pusher cannot run using the "simple sync" mode mentioned in the puller description from this file.

Because it hosts a webserver, the pusher runs as a daemon and never exists unless it `panic`s because of an error, or it is killed (e.g. with `Ctrl+C`).


## Build

The manager can be built using [gb](https://getgb.io), which can be installed by running

```bash
go get github.com/constabulary/gb/...
```

It can then be built by cloning this repository and running

```bash
cd grafana-dashboards-manager
gb build
```

Once built, binaries are located in the `bin` directory (which is created by `gb` if it doesn't exist).

## Run

To run either the puller or the pusher, simply execute the corresponding binary

```bash
./puller
```

or

```bash
./pusher
```

Of course, this command line call may depend on the location and name of the binaries.

You can specify a configuration file via the command line flag `--config`, which works with both the puller and pusher. For example, here's how the full call should look like when passing a configuration file path to the puller:

```bash
./puller --config /etc/grafana-dashboards-manager/config.yaml
```

If the `--config` flag isn't present in the command line call, it will default to a `config.yaml` file located in the directory from where the call is made.

The pusher can also be called with the `--delete-removed` flag which will allows it to check for dashboards which files were removed from the Git repository and delete them from Grafana.

## Configure

To run either the puller or the pusher, you will need a configuration file first. The simplest way to create one is to copy the `config.example.yaml` file at the root of this repository and replace all of the placeholder values with your own.

Since all the keys are documented as comments in the `config.example.yaml` file, there won't be any more documentation about them in this README file.
