import React, { useMemo } from 'react';
import { css } from '@emotion/css';
import { GrafanaTheme2 } from '@grafana/data';
import { useStyles2, Text, Alert, Stack } from '@grafana/ui';

interface Props {
  subject?: string;
  html?: string;
  from?: string;
  to?: string;
  error?: string;
  height?: number;
}

/**
 * Shows a rendered email the way a mail client would: header lines, then the
 * HTML in a sandboxed frame that cannot run scripts or reach Grafana.
 */
export function EmailPreview({ subject, html, from, to, error, height = 560 }: Props) {
  const s = useStyles2(getStyles);
  // Links open in a new tab instead of inside the frame.
  const doc = useMemo(() => (html ? `<base target="_blank">${html}` : ''), [html]);
  return (
    <div className={s.mail} data-testid="email-preview">
      <div className={s.head}>
        <Text weight="medium">{subject || '(no subject)'}</Text>
        <Text variant="bodySmall" color="secondary">
          From {from || 'Grafana Alerts'} to {to || 'default recipients'}
        </Text>
      </div>
      {error ? (
        <div className={s.error}>
          <Alert severity="error" title="Template error">
            <Stack direction="column">
              <code className={s.code}>{error}</code>
            </Stack>
          </Alert>
        </div>
      ) : (
        <iframe
          title="Email preview"
          className={s.frame}
          style={{ height }}
          sandbox="allow-popups allow-popups-to-escape-sandbox"
          srcDoc={doc}
        />
      )}
    </div>
  );
}

const getStyles = (theme: GrafanaTheme2) => ({
  mail: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    overflow: 'hidden',
    background: theme.colors.background.primary,
  }),
  head: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.25),
    padding: theme.spacing(1, 1.5),
    borderBottom: `1px solid ${theme.colors.border.weak}`,
  }),
  frame: css({
    display: 'block',
    width: '100%',
    border: 0,
    // Emails are designed for a light background.
    background: '#ffffff',
  }),
  error: css({ padding: theme.spacing(1.5) }),
  code: css({
    whiteSpace: 'pre-wrap',
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
  }),
});
