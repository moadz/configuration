# gimme

## Setup

```bash
make install
mkdir -p ~/.config/gimme && cp gimme.yaml ~/.config/gimme/gimme.yaml
```

## Running

```bash
gimme connects to a private AppSRE cluster by:
1. Ensuring VPN is connected via Viscosity
2. Adding network route for the cluster CIDR
3. Starting sshuttle tunnel through bastion
4. Opening console, prometheus, and alertmanager in browser

Ensure you are on MacOS, and have Viscosity installed.
Also ensure you have a valid gimme.yaml config file.

Arguments:
  env     Environment: stage, int, or prod
  region  Region code (e.g., ue1, uw2)

Examples:
  gimme stage ue1    # Connect to staging cluster in us-east-1
  gimme int uw2      # Connect to integration cluster in us-west-2

Usage:
  gimme <env> <region> [flags]

Flags:
  -c, --config string            Path to gimme.yaml config file (default "/Users/samukher/.config/gimme/gimme.yaml")
  -h, --help                     help for gimme
  -w, --sshuttle-wait duration   Max time to wait for sshuttle to connect (default 1m0s)
```