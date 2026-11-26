# Examples

Ready-to-use templates and configuration. Copy what you need.

## Templates

| File | Use it for |
| --- | --- |
| [`templates/query-rows.html`](templates/query-rows.html) | SQL alert rules that return one row per problem (orders, hosts, customers). Shows one table row per alert, with the columns you choose. |
| [Cards](../pkg/templates/starters/default.html) | The built-in default: one card per alert with status, severity, values, labels and links. |
| [Compact table](../pkg/templates/starters/compact.html) | One line per alert, for busy inboxes. |
| [Summary](../pkg/templates/starters/summary.html) | Big status and counts, for managers and stakeholders. |
| [Blank](../pkg/templates/starters/blank.html) | A starting point for your own. |

The four starters are also offered in the plugin under **New template**.

### Use a template from this folder

1. Open **Beautify Alert, Templates, New template**, give it a name (for example `query-rows`), start from **Blank**, and click **Create template**.
2. Replace the HTML with the file's contents.
3. Set a subject, for example `{{ .Name }} ({{ .FiringCount }} open)`.
4. For `query-rows.html`, change the line `{{- $columns := list "order_id" "store" "status" -}}` to the column names your query returns.
5. Save, and add `email_template=query-rows` to the alert rule.

### Matching alert rule for `query-rows.html`

Any SQL data source works. The query returns one row per problem: string columns become labels, and one number column is the value.

```sql
SELECT order_id, store, status, 1 AS value
FROM orders
WHERE status IN ('PICKED', 'SCHEDULED')
  AND allocation_raised = 0
```

In the rule:

- Format the query as **Table**.
- Condition: `A > 0`.
- Labels: `email_template = query-rows`.
- No data and error handling: **Normal** for no data, so an empty result is not an alert.

## Provisioning

[`provisioning/beautify-alert.yaml`](provisioning/beautify-alert.yaml) sets the SMTP server, recipients and a template from a file. Copy it to Grafana's `provisioning/plugins/` directory and edit it. See [provisioning](../docs/provisioning.md).

## Share yours

Made a template others could use? Open a pull request that adds it to `examples/templates/` with a comment at the top saying what it is for. See [CONTRIBUTING.md](../CONTRIBUTING.md).
