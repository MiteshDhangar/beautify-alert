# Routing alerts

Two separate decisions decide what email an alert becomes:

1. **Grafana decides whether the alert goes to Beautify Alert,** with an alert rule's contact point or a notification policy.
2. **Beautify Alert decides the template and the recipients,** from the alert's labels.

## 1. Send alerts to Beautify Alert

Pick one way per rule.

**In the alert rule** (simplest). In the rule's **Configure notifications** section, choose the contact point **Beautify Alert**.

**With a notification policy** (for many rules). In Alerting, Notification policies, add a child policy:

| Matcher | Contact point | Use it for |
| --- | --- | --- |
| `email_template =~ .+` | Beautify Alert | Every rule that names a template |
| `team = finance` | Beautify Alert | Everything a team owns |
| `severity = critical` | Beautify Alert | Only critical alerts |

Turn on **Continue matching** if the alerts should also keep going to your other contact points, for example while you compare the old and new emails.

**Grouping matters.** Grafana groups alerts into one notification by the policy's **Group by** labels (by default the rule name and folder). One notification becomes one email per template and recipient list, so all instances of a rule usually arrive in one email.

## 2. Template and recipients

For **each alert** in a notification, the plugin picks, first match wins:

| | 1 | 2 | 3 | 4 | 5 |
| --- | --- | --- | --- | --- | --- |
| **Template** | `?template=` on the webhook URL | `email_template` label | `email_template` annotation | `default` | |
| **Recipients** | `?to=` on the webhook URL | `email_to` label | `email_to` annotation | the template's recipients | default recipients |

Alerts that end up with the same template and the same recipients are sent as one email. If an alert names a template that does not exist, it uses `default` and the delivery log shows a warning.

The label names can be changed on the Configuration page.

### Examples

A notification with four alerts:

| Alert | Labels | Email |
| --- | --- | --- |
| Payment failed ORD-1 | `email_template=payments` | Email A: `payments` template to the template's recipients (for example finance@) |
| Payment failed ORD-2 | `email_template=payments` | Email A (same template, same recipients) |
| Payment failed ORD-3 | `email_template=payments`, `email_to=cfo@example.com` | Email B: `payments` template to cfo@ |
| Disk full | none | Email C: `default` template to the default recipients |

Several recipients: `email_to=ops@example.com, dba@example.com`. Addresses that are not valid are skipped with a warning in the delivery log.

### Webhook URL overrides

`?template=` and `?to=` on the contact point's URL apply to every alert sent through that contact point. Use them to make a second contact point with a fixed layout or audience:

```
http://localhost:3000/api/plugins/prudentbee939-beautifyalert-app/resources/alert?template=summary&to=management@example.com
```

Create it like the main contact point (Alerting, Contact points, Webhook, with the same Bearer token), then route to it from a notification policy.

## Labels from SQL queries

For SQL data sources (MySQL, PostgreSQL, SQL Server), a rule whose query returns a table creates one alert per row, and every string column becomes a label. For example:

```sql
SELECT order_id, store, 1 AS value
FROM orders
WHERE status = 'PICKED' AND allocation_raised = 0
```

creates alerts with the labels `order_id` and `store`. The [query rows example](../examples/templates/query-rows.html) template lists them as a table, one row per alert.

If the rule only returns rows during a time window, turn off **Resolved-only emails** on its template. Otherwise, when the window closes, every alert resolves at once and you get an email saying everything is fixed.

## Grafana's own error alerts

When a rule's query fails, Grafana sends an alert named `DatasourceError` (or `DatasourceNoData` when it returns nothing and the rule is set to alert on no data). These keep the rule's labels, so they go to the same template. Templates can show them differently with `.Problem` and `.Error`; the starter templates already do. See the [template reference](templates.md#grafanas-error-alerts).
