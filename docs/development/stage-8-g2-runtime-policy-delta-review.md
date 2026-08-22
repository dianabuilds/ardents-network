# Stage 8 G2 runtime, command, and policy delta review

Status: **S8.0 factual delta review; not a target-design decision.** This
temporary record completes the G2 source-delta review for F041--F067 at the
Stage 8 entry `1cf7100da3ada32ba53abb51201aaf7b6183a3da`.

## Method

The entry delta changes Route actor validation and tests, Node tests, and the
Update storage-admission implementation. It does not refactor Application IPC,
Bridge, Node responsibility, local-duty ownership, Resource, WebTunnel,
planfile, command-plan composition, Make targets, architecture gates, or
historical laboratories. The focused current Module/command diagnostics for
Route, Bridge, Node, local duty, Resource, WebTunnel, planfile, and composition
were run at the entry; their green behavior does not decide an ownership,
platform, or claim question.

## Finding disposition

| Findings | Entry disposition |
|---|---|
| F041--F044 | **Confirmed; open.** No relevant delta changes Application socket peer authority, session/revocation ownership, timing-selected terminal result framing, or the tracer-only public commands. A Broker/explicit Application compatibility decision remains necessary. |
| F045--F048 | **Confirmed; open after direct delta inspection.** The Route delta validates actors before side effects but retains the role union, concrete State/plan coupling, stage sequence orchestration, and combined endpoint/Node execution. It is not a Route product-Interface replacement. |
| F049--F052 | **Confirmed; open.** Bridge callback/State exposure, Node/probe boundary, and local-duty physical-root duplication are unchanged. `entry`, private Node probe, and a narrow network-duty owner remain S8.3 design inputs. |
| F053--F054 | **Confirmed; open.** No Resource delta changes non-Linux all-zero native measurement or independent mutable Guards. A platform-admitted, synchronized process Resource manager is still required if preserved. |
| F055--F057 | **Confirmed; open.** WebTunnel child/cleanup, address/root race, and public Adapter choreography are unchanged. No Windows cleanup claim is implied. |
| F058--F061 | **Confirmed; open.** Commands still reconstruct stage facts from plans; `planfile`, display-text classification, and cleanup-error observation retain the prepared problems. These are S8.2/S8.3 policy and composition work, not package renames. |
| F062 | **Confirmed after direct current inspection.** Windows admission only confirms that a DACL exists and uses allocation unit `1`; it neither proves owner-only ACEs nor conservatively accounts for cluster allocation. Windows activation remains unsupported unless this is repaired and tested. |
| F063 | **Confirmed; open limitation.** Update storage admission is one lock-held point observation, explicitly not a reservation. Any target must preserve later write/sync failure handling and must not claim capacity reservation without new evidence. |
| F064--F067 | **Confirmed; open.** Make's negative lab filters, stage-shaped test/evidence portfolio, mixed architecture gate, and 14 lab roots/six lab commands are unchanged. S8.2 must assign every retained test/evidence artifact one profile and S8.1/S8.4 must decide each historical surface before removal. |

## Completion of the G2 delta review

Together with the Release/Update, Network State, naming, and Service reviews,
this record gives all G2 F001--F067 an entry disposition:

- exact historical statements invalidated by the Update delta are F008 and the
  former recovery portion of F011;
- every other finding remains an open source-anchored design, research,
  compatibility, platform, testing, or product-disposition input; and
- a green current test cannot close a finding that its own oracle omits or
  contradicts.

The completed review authorizes no mutation. S8.1 decides which affected
surfaces survive; S8.2/S8.3 select their policy, Interface, format, and
compatibility replacements.
