# Secrets

A secret travels only as far as its tier allows: kept on the host, lent to a VM as a copy, or bound inside one VM.

| Section         | Answers                                                  |
| --------------- | -------------------------------------------------------- |
| The three tiers | What each tier holds and what delete does to it          |
| The lend list   | Which names may be lent, and `secrets lend` and `recall` |
| Where to revoke | Where each service lets you invalidate a token           |
| Studio in a VM  | What lending needs inside the VM                         |
| Claude in a VM  | Logging Claude Code in on a VM                           |
| Accounts        | Who has access to what, and where each right comes from  |

## The three tiers

| Tier  | Lives                                         | Examples                                                       | What `machines delete` does                                        |
| ----- | --------------------------------------------- | -------------------------------------------------------------- | ------------------------------------------------------------------ |
| kept  | the host only                                 | the owner's GitHub token, Apple ID, `AGENT_PASSWORD`           | nothing: it never leaves                                           |
| lent  | the host; a copy in the VM's credential store | `session:*` browser sessions; the Claude token                 | the copy goes with the disk; the secret stays valid at its service |
| bound | inside one VM                                 | its ssh host keys, its `github-login` token, a `claude /login` | the disk goes; a token stays valid at its service until revoked    |

Deleting a VM is not revoking a token. Use fine-grained, expiring tokens in a VM, and revoke at the service.

## The lend list

`lend_list` in `preferences.json` names what may be lent: `session:*` names and the one name `operator:claude:oauth-token`. Anything else is refused with a hint that other secrets wait for the secret manager ([TODO](../../TODO.md)).

```sh
station secrets allow session:openpanel
station secrets lend --to api            # the whole lend list, for 12h; lending again refreshes
station secrets lend --to api --for 4h   # a term from 1m to 7d
station secrets recall --from api        # take the copies back out, after a y/n (--yes off a terminal)
station secrets disallow session:openpanel   # off the list, and recalled from every VM
station secrets                          # names only, never a value
```

A lent copy is a snapshot: later captures on the host do not reach it until you lend again. Every lend expires: the VM's record keeps, per name, when it was lent and until when (`secrets --json` lists them under `lends`), lending again resets the term, and at its until the engine recalls the copy from a VM whose guest is connected (at its start and every minute; a VM still connecting, or a lasting VM stopped past its until, is recalled as its guest connects, before it is reported ready), logging `<vm> lend: recalled <name> (expired)`; a recall that fails keeps the entry and is logged once an hour. A record from before terms existed reads each name as lent at the file's last change for 12h. Values pass between the credential stores through a pipe, never through argv and never to disk on the host. A lend and a recall run inside the VM over the engine's one connection to it, so they need the engine up and the VM's guest connected.

## Where to revoke

| Secret                             | Revoke at                                                                                      |
| ---------------------------------- | ---------------------------------------------------------------------------------------------- |
| A VM's GitHub token                | github.com, your account page for developer tokens and authorized apps                         |
| A lent `session:*` browser session | log out of that service in the host browser; the service's session list shows the others       |
| The Claude token                   | claude.ai, your account page for sessions and tokens; then `station settings off claude-token` |

## Studio in a VM

Lending uses the studio CLI's credential store on both ends (`studio credentials export`, `import`, `delete`), because that store has one owner. A VM without `studio` on its PATH makes `secrets lend` fail with a hint to this section. How the studio CLI reaches a VM is not designed yet ([TODO](../../TODO.md)); until then, lending is judged with stubs.

## Claude in a VM

A VM cannot open a browser on your host to log in, so the host lends it a token.

1. On the host, once: `claude setup-token`. It opens a browser OAuth and prints a long-lived subscription token.
2. `station settings apply claude-token`. A form asks for the token, the OS credential store keeps it as `operator:claude:oauth-token` (in the keychain on a Mac), and the name joins the lend list.
3. `station secrets lend --to <vm>` copies it.
4. Inside the VM, `station shell zsh` exports `CLAUDE_CODE_OAUTH_TOKEN` from the VM's credential store at every shell start. With no `studio`, no name or any failure it prints nothing: a shell start never fails.

`station secrets recall` and `machines delete` remove the VM's copy; `station settings off claude-token` deletes the host's and takes the name off the lend list. Revoke the token itself at claude.ai.

The alternative is a bound login: run `claude /login` inside a lasting VM. The login lives and dies with that VM and is never lent.

## Accounts

`station accounts` (the TUI's Accounts tab) shows who has access to what: each provider's identity on this host, then on each of its VMs, and where the right comes from: `own` (a login done on that machine), `lent` (a copy of a lend-list name, with its `until`), or both. A lend in its last hour is `◐ expiring` with the re-lend command; an expired one the engine could not recall is `✗ failing` with the recall command. A VM is read by its own guest over the engine's session, so no provider's output ever leaves the VM: only the parsed identities and scopes do. Each provider's read is an allowlist of named fields (`src/domain/accounts/parse.go`), and every kept value that looks like a token (`gh*_`, `github_pat_`, `sk-ant-`, `npm_`, `xox*-`, a JWT, 32 and more characters of base64 or hex) is replaced by `?`.

## Related

- [Machines](machines.md): `delete`.
- [Configuration](configuration.md): `claude-token`.
