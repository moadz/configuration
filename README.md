# Red Hat Observability Service

This project holds the configuration files for our internal Red Hat Observability Service based on [Observatorium](https://github.com/observatorium/observatorium).

See our [website](https://rhobs-handbook.netlify.app/) for more information about RHOBS.

## Requirements

* Go
* [Mage](https://magefile.org/) (`go install github.com/magefile/mage@latest`)

### macOS

* findutils (for GNU xargs)
* gnu-sed

Both can be installed using Homebrew: `brew install gnu-sed findutils`. Afterwards, update the `SED` and `XARGS` variables in the Makefile to use `gsed` and `gxargs` or replace them in your environment.

## Hot fixing Thanos Operator
When a critical issue is found in production, we sometimes need to hotfix the deployed configuration without 
going through the full development and deployment cycle. The steps below outline the process to do so.
We use production as an example, but the same steps apply to stage or any other environment in terms of process.
The process can also be used for rolling out a new version of the operator if needed.

Currently, we build directly from [upstream](https://github.com/thanos-community/thanos-operator).
This works for now as we are maintainers of the project, but in future we might need to fork it.

1. Create and merge a PR in the upstream repository with the fix.
2. Go to our [Konflux fork](https://github.com/rhobs/rhobs-konflux-thanos-operator) where we build from a submodule.
3. Run this [workflow](https://github.com/rhobs/rhobs-konflux-thanos-operator/actions/workflows/update-submodules.yml) targeting `main`.
4. Merge the [generated PR](https://github.com/rhobs/rhobs-konflux-thanos-operator/pulls).
5. Visit [quay.io](https://quay.io/repository/redhat-services-prod/rhobs-mco-tenant/rhobs-thanos-operator?tab=tags&tag=latest) to ensure the new image is built and available.
6. Run `mage sync:operator thanos latest`
7. Run `mage build:environment production` to generate the manifests for production environment.

## Building RHOBS Cells with Mage

This repository leans heavily on [Mage](https://magefile.org/) to build various components of RHOBS. You can find the available Mage targets by running:

```bash
mage -l
```

### Synchronizing Operators

Because we ship operators and their Custom Resource Definitions (CRDs) as part of our RHOBS service, 
we need to keep them in sync with the versions deployed in our clusters. 
We are further complicated by the reqirement to ship images built on Konflux so we need to maintain a mapping between 
upstream operator versions and our Konflux-built images.

To facilitate this, we provide a Mage target `mage sync:operator` that automates the synchronization process.
This allows us to keep the image versions in sync with the CRDs they support.

The target requires two parameters:
1. `operator`: The name of the operator to synchronize and should be one of (`thanos`).
2. The commit hash for the fork we want to sync to or "latest" to sync to the latest commit on the supported branch.

For `thanos`, this is the commit hash on https://github.com/rhobs/rhobs-konflux-thanos-operator
An example is shown below:

```bash
mage sync:operator thanos latest
```
This will update some internal configuration and sync the dependency in go modules.
You can now proceed to build for a specific environment using `mage build:environment <env>`.

## Usage

This repository contains [Jsonnet](https://jsonnet.org/) configuration that allows generating Kubernetes objects that compose RHOBS service and its observability.

### RHOBS service

The jsonnet files for RHOBS service can be found in [services](./services) directory. In order to compose *RHOBS Service* we import many Jsonnet libraries from different open source repositories including [kube-thanos](https://github.com/thanos-io/kube-thanos) for Thanos components, [Observatorium](https://github.com/observatorium/observatorium) for Observatorium, Minio, Memcached, Gubernator, Dex components, [thanos-receive-controller](https://github.com/observatorium/thanos-receive-controller) for Thanos receive controller component, [parca](https://github.com/parca-dev/parca) for Parca component, [observatorium api](https://github.com/observatorium/api) for API component, [observatorium up](https://github.com/observatorium/up) for up component,  [rules-objstore](https://github.com/observatorium/rules-objstore) for rules-objstore component.

Currently, RHOBS components are rendered as [OpenShift Templates](https://docs.openshift.com/container-platform/latest/openshift_images/using-templates.html) that allows parameters. This is how we deploy to multiple clusters, sharing the same configuration core, but having different details like resources or names.

> This is why there might be a gap between vanilla [Observatorium](https://github.com/observatorium/observatorium) and RHOBS. We have plans to resolve this gap in the future.

Running `make manifests` generates all required files into [resources/services](./resources/services) directory.

### Unified Templates

Some services use unified, environment-agnostic templates that can be deployed across all environments using template parameters. For example, the synthetics-api service provides a single template that works for all environments:

```bash
# Generate unified synthetics-api template
mage unified:syntheticsApi

# Generate all unified templates
mage unified:all

# List available unified templates
mage unified:list

# Deploy to different environments using parameters
oc process -f resources/services/synthetics-api-template.yaml \
  -p NAMESPACE=rhobs-stage \
  -p IMAGE_TAG=latest | oc apply -f -

oc process -f resources/services/synthetics-api-template.yaml \
  -p NAMESPACE=rhobs-production \
  -p IMAGE_TAG=v1.0.0 | oc apply -f -
```

This approach reduces template duplication and ensures consistency across environments while maintaining deployment flexibility.

### Observability

Similarly, in order to have observability (alerts, recording rules, dashboards) for our service we import mixins from various projects and compose all together in [observability](./observability) directory.

Running `make prometheusrules grafana` generates all required files into [resources/observability](./resources/observability) directory.

### Updating Dependencies

Up-to-date list of jsonnet dependencies can be found in [jsonnetfile.json](./jsonnetfile.json). Fetching all deps is done through `make vendor_jsonnet` utility.

To update a dependency, normally the process would be:

```console
make vendor_jsonnet # This installs dependencies like `jb` thanks to Bingo project.
JB=`ls $(go env GOPATH)/bin/jb-* -t | head -1`

# Updates `kube-thanos` to master and sets the new hash in `jsonnetfile.lock.json`.
$JB update https://github.com/thanos-io/kube-thanos/jsonnet/kube-thanos@main

# Update all dependancies to master and sets the new hashes in `jsonnetfile.lock.json`.
$JB update
```

## App Interface

Our deployments our managed by our Red Hat AppSRE team.

### Updating Dashboards

**Staging**: Once the PR containing the dashboard changes is merged to `main` it goes directly to stage environment - because the `telemeter-dashboards` resourceTemplate refers the `main` branch [here](https://gitlab.cee.redhat.com/service/app-interface/-/blob/master/data/services/rhobs/telemeter/cicd/saas.yaml).

**Production**: Update the commit hash ref in [the saas file](https://gitlab.cee.redhat.com/service/app-interface/-/blob/master/data/services/rhobs/telemeter/cicd/saas.yaml) in the `telemeterDashboards` resourceTemplate, for production environment.

### Prometheus Rules and Alerts

Use `synchronize.sh` to create a MR against `app-interface` to update dashboards.

### Components - Deployments, ServiceMonitors, ConfigMaps etc...

**Staging**: update the commit hash ref in [`https://gitlab.cee.redhat.com/service/app-interface/blob/master/data/services/telemeter/cicd/saas.yaml`](https://gitlab.cee.redhat.com/service/app-interface/blob/master/data/services/telemeter/cicd/saas.yaml)

**Production**: update the commit hash ref in [`https://gitlab.cee.redhat.com/service/app-interface/blob/master/data/services/telemeter/cicd/saas.yaml`](https://gitlab.cee.redhat.com/service/app-interface/blob/master/data/services/telemeter/cicd/saas.yaml)

## CI Jobs

Jobs runs are posted in:

`#sd-app-sre-info` for grafana dashboards

and

`#team-monitoring-info` for everything else.

## Troubleshooting

1. Enable port forwarding for a user - [example](https://gitlab.cee.redhat.com/service/app-interface/-/blob/ee91aac666ee39a273332c59ad4bdf7e0f50eeba/data/teams/telemeter/users/fbranczy.yml#L14)
2. Add a pod name to the allowed list for port forwarding - [example](https://gitlab.cee.redhat.com/service/app-interface/-/blob/ee91aac666ee39a273332c59ad4bdf7e0f50eeba/resources/app-sre/telemeter-production/observatorium-allow-port-forward.role.yaml#L10)
