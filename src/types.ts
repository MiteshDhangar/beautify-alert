// Types shared with the Go backend (pkg/config, pkg/alert, pkg/history).

export interface EmailTemplate {
  name: string;
  description?: string;
  recipients?: string;
  subject: string;
  html: string;
  inlineCss?: boolean;
  skipResolved?: boolean;
}

export interface Starter extends EmailTemplate {
  title: string;
}

export type SmtpSecurity = 'starttls' | 'tls' | 'none';

export interface Delivery {
  provider: 'smtp';
  fromAddress: string;
  fromName: string;
  replyTo?: string;
  smtp: {
    host: string;
    port: number;
    username: string;
    security: SmtpSecurity;
    skipVerify: boolean;
  };
}

export interface Routing {
  templateLabel: string;
  recipientsLabel: string;
}

/** The plugin's jsonData. */
export interface Settings {
  delivery: Delivery;
  defaultRecipients: string;
  routing: Routing;
  templates: EmailTemplate[];
  /** Templates as base64 gzip-compressed JSON; replaces templates when set. */
  templatesGz?: string;
  threading?: boolean;
  maxAlertsPerEmail?: number;
  /** Comma-separated domains every email is limited to; empty allows any. */
  allowedRecipientDomains?: string;
  maxRecipientsPerEmail?: number;
  maxEmailsPerNotification?: number;
}

export interface Rendered {
  subject: string;
  html: string;
  text: string;
}

export interface HistoryEntry {
  id: string;
  time: string;
  source: 'alert' | 'test' | 'resend';
  template: string;
  to: string[];
  subject: string;
  alertStatus?: string;
  alerts: number;
  ok: boolean;
  error?: string;
  warnings?: string[];
  skipped?: string;
  message?: { to: string[]; subject: string; html: string; text: string };
}

export interface ApiError {
  error: string;
  part?: 'subject' | 'html';
}
