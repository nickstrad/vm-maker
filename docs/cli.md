# vm-maker CLI

This is the proposed v1 command contract. The repository currently contains a hello-world
scaffold; these commands are not implemented yet.

vm-maker is a stateless CLI for lab VMs on Hetzner Cloud and DigitalOcean. Every invocation
reads the provider API. Valid credentials are sufficient to rediscover VMs created by this CLI,
a Bash script, another tool, or the provider UI. Optional config stores preferences and limits,
never VM inventory. There is no TTL, reaper, scheduler, saved plan, or local state database.

## Command surface

```text
vm-maker create <NAME> (--type TYPE | --vcpu N --memory GB [--disk GB])
                [--region REGION] [--image IMAGE] [--ssh-key ID]...
                [--user-data FILE] [--dry-run] [--wait]
vm-maker list [--provider hetzner|digitalocean|all] [--region REGION]
vm-maker show <ID>
vm-maker stop <ID> [--dry-run] [--yes] [--wait]
vm-maker start <ID> [--dry-run] [--wait]
vm-maker delete <ID> [--dry-run] [--yes] [--wait]
vm-maker catalog types [--region REGION] [--arch x86|arm] [--within-limits]
vm-maker catalog regions
vm-maker catalog images [--region REGION] [--arch x86|arm]
vm-maker config show
vm-maker config check
vm-maker version
```

Use `--provider hetzner|digitalocean` on any provider command, or set `defaultProvider` in
config. With no configured default, provider selection is required. `all` is supported only
by `list`; mutations always select exactly one provider and one ID. Create always creates
one VM. There is no count flag, bulk operation, wildcard target, or name-based mutation.

Names are display names, not identities: duplicate names are allowed by some providers.
The identity is `(provider, ID)`, with IDs represented as opaque strings. Copy the ID from
`list` into `show`, `stop`, `start`, or `delete`.

### Discover and inspect

`list` returns all VMs visible to the selected credentials across regions unless filtered.
It includes powered-off VMs and VMs without vm-maker labels. No import, adoption, or saved
creation record is needed. Hetzner credentials scope this to a project; DigitalOcean results
are scoped to the account/team accessible to the token.

Text columns: `PROVIDER ID NAME TYPE REGION STATUS RAM_GB DISK_GB IPV4 IPV6`.
`show` adds image, creation time, provider labels/tags, attached resource references when
available, and the provider's raw status. Missing values print `-`, never guessed values.
RAM and disk reflect allocated capacity, not guest usage.

`list --provider all` requires credentials for both providers. If one fails, return the
successful provider's rows, identify the failed provider, and exit nonzero. A pagination
limit reached is also an explicit incomplete result, never a silently complete list.

### Create

Region and image must be supplied by flags or provider-specific config. SSH key IDs reference
keys already registered with the provider; v1 does not upload keys. `--user-data` sends the
supplied UTF-8 file after checking provider size limits. Cloud-init files can be passed this
way; templates, secret fetching, and guest readiness checks are outside v1.

Choose an explicit type or resource minimums, never both. `--vcpu`, `--memory`, and `--disk`
are minimum requirements when selecting a type. Disk defaults to no minimum. The resolver
chooses the smallest compatible type by RAM, then vCPU, then disk, then stable type ID.
It does not optimize by price. Validate the actual selected type against resource ceilings,
including its bundled disk, even when `--disk` is omitted. No matching type means refusal;
never silently relax a minimum or increase a ceiling.

The provider catalog supplies type, region, image, and architecture compatibility. Values
in examples are illustrative; discover current identifiers using `catalog`.

`--dry-run` fetches the live catalog and prints the resolved type and redacted request without
sending a mutation. A later real invocation resolves again; dry-run is not a saved approval
or a promise that catalog availability will remain unchanged.

### Stop, start, and delete

`stop` requests graceful guest shutdown. `start` powers the VM on. Stop is the available
pause-like operation; it does not suspend RAM or preserve running processes. There is no
implicit hard power-off if graceful shutdown fails. If already in the requested stable
power state, return success without sending an action. Transitional states produce a clear
conflict error rather than an implicit sequence of actions.

Powered-off VMs remain billable on both providers. See [Hetzner billing FAQ](https://docs.hetzner.com/cloud/billing/faq/)
and [DigitalOcean Droplet pricing](https://docs.digitalocean.com/products/droplets/details/pricing/).

`delete` destroys the selected VM and its root disk. It uses the ordinary single-VM deletion
endpoint, not a delete-by-tag or recursive destruction endpoint. Show attached resources
when available and state that independently billed volumes, snapshots, or IP resources may
remain; provider-native deletion behavior determines what is removed with the VM.

`stop` and `delete` fetch the current VM, show its provider, ID, name and intended action,
and ask for confirmation. `--yes` skips the prompt; a non-TTY without `--yes` exits 7.
`--dry-run` never prompts. A missing ID exits 6. Ownership labels are never required.
Resource creation limits do not restrict inspection, stopping, starting, or deletion of
existing VMs, even if they exceed current limits.

Mutations return an accepted operation and action ID when available. `--wait` polls for the
requested provider state or confirmed deletion: default deadline 3 minutes, maximum 10
minutes via `--timeout`. This checks provider state, not SSH or cloud-init readiness. Timeout
does not undo the request; output retains the VM/action ID so a later `show` can inspect it.

## Limits and optional configuration

Default creation ceilings are **8 GB RAM and 128 GB bundled storage per VM**. These are
editable config values, not compiled-in resource caps. Raise them later by editing config;
no code change is needed. Default vCPU ceiling is 8. Positive finite values are required,
and actual provider offerings remain the final compatibility constraint.

RAM uses each provider's plan capacity: DigitalOcean memory in MiB is divided by 1024;
Hetzner's GB field is used directly. Disk uses the provider's advertised GB field. These are
plan sizing units, not promises about formatted guest capacity.

```toml
# Optional ./vm-maker.toml, or ~/.config/vm-maker/config.toml
# --config PATH selects a specific file. No file is required.
defaultProvider = "hetzner"

[limits]
maxVcpu = 8
maxMemoryGb = 8
maxDiskGb = 128

[providers.hetzner]
tokenEnv = "HCLOUD_TOKEN"
region = "fsn1"
image = "ubuntu-24.04"
sshKeys = []

[providers.digitalocean]
tokenEnv = "DIGITALOCEAN_TOKEN"
region = "fra1"
image = "ubuntu-24-04-x64"
sshKeys = []
```

`config show` prints effective preferences, limits, and their source, with no token values.
`config check` validates offline and needs no credentials. Unknown config keys fail rather
than being ignored, including removed expiry settings. Flags override provider defaults;
resource limits come only from defaults or config. Tokens come from environment variables.

There is no managed-VM count limit in v1: this tool manages any visible VM, and inventory
checks cannot enforce a reliable account-wide cap across concurrent tools. The one-VM-per-
create contract still bounds each invocation.

## Shared options and output

| Option | Behavior |
| --- | --- |
| `--provider hetzner|digitalocean` | Select credential scope; `list` also accepts `all`. |
| `--config PATH` | Read this optional preferences file; explicit missing path is an error. |
| `--output text|json` | Default text; JSON is a versioned envelope. |
| `--dry-run` | Supported on create/start/stop/delete; live reads only. |
| `--yes` | Skip stop/delete confirmation. |
| `--wait` / `--timeout DURATION` | Mutation completion polling; timeout requires wait. |
| `--verbose` | Redacted diagnostics to stderr. |

JSON stdout contains exactly one object: `schemaVersion`, `ok`, `data`, and `errors`.
Errors contain a stable code, message, and provider/VM/action IDs when available. Multi-provider
partial results use `ok: false` with successful rows in `data` and failures in `errors`.
Prompts and diagnostics go to stderr. Tokens and user-data never appear in diagnostic or
dry-run output; user-data is represented by its byte length and content hash.

| Exit | Meaning |
| ---: | --- |
| 0 | Success, including a valid dry-run or already-requested stable power state. |
| 1 | Internal defect. |
| 2 | Invalid arguments or config. |
| 3 | Incompatible request or provider state conflict. |
| 4 | Resource ceiling exceeded. |
| 5 | Authentication, provider, transport, or incomplete pagination error. |
| 6 | VM ID not found in the selected credential scope. |
| 7 | Confirmation declined or unavailable without `--yes`. |
| 9 | Completion deadline exceeded; request may still complete. |

## Example workflow

```bash
# Rediscover a VM created yesterday by a Bash script.
vm-maker list --provider hetzner
vm-maker show 51234567 --provider hetzner
vm-maker stop 51234567 --provider hetzner --wait
vm-maker start 51234567 --provider hetzner --wait
vm-maker delete 51234567 --provider hetzner --yes --wait

# Discover both platforms with both tokens available.
vm-maker list --provider all --output json

# Review a lab VM before creating it; defaults must provide region and image.
vm-maker create scratch --provider hetzner --vcpu 2 --memory 4 --disk 40 --dry-run
vm-maker create scratch --provider hetzner --vcpu 2 --memory 4 --disk 40 --wait

# Display types that fit the current ceilings, then select a current type ID.
vm-maker catalog types --provider digitalocean --within-limits
```

A request with `--memory 16` fails against the default 8 GB ceiling. A type with 8 GB RAM
and 160 GB bundled disk also fails against the 128 GB ceiling. Change `[limits]` to permit
larger labs later. No limit prevents you from listing or deleting either existing VM.

## Scope

V1 covers create, list, show, stop, start, delete, catalog discovery, and config inspection.
Resize, rename, label editing, presets, declarative reconciliation, saved plan application,
volume management, and cross-provider migration can be separate future additions. Existing
provider metadata is displayed and preserved; no vm-maker metadata is necessary for discovery
or permission to act on a VM.
