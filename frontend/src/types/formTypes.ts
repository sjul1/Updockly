export interface ConnectionFormState {
  name: string;
  engine: "postgres" | "mysql" | "mongodb" | "redis";
  host: string;
  port: number;
  mode: "direct" | "docker" | "systemd";
  target: string;
  username: string;
  password: string;
  database: string;
  tags: string;
  secure: boolean;
  notes: string;
}

export interface ScheduleFormState {
  name: string;
  connectionId: string;
  frequency: "hourly" | "daily" | "weekly" | "custom";
  cronExpression: string;
  intervalMinutes: number;
  retentionDays: number;
  storageClass: string;
  compression: string;
  encryption: boolean;
  enabled: boolean;
  startAt: string;
}

export interface RestoreFormState {
  backupId: string;
  targetConnectionId: string;
  targetDatabase: string;
  targetHost: string;
  targetPort: number;
  notes: string;
  overwriteExisting: boolean;
}

export type BackupDestinationType =
  | "local"
  | "webdav"
  | "gdrive"
  | "onedrive";

export interface BackupDestinationSettings {
  type: BackupDestinationType;
  webdavUrl: string;
  webdavUsername: string;
  webdavPassword: string;
  googleCredentials: string;
  onedriveTenant: string;
  onedriveClientId: string;
  onedriveClientSecret: string;
}

export interface SMTPSettingsState {
  host: string;
  port: number;
  user: string;
  password: string;
  from: string;
  tls: boolean;
  enabled: boolean;
}

export interface NotificationSettingsState {
  webhookUrl: string;
  discordToken: string;
  discordChannel: string;
  onSuccess: boolean;
  onFailure: boolean;
  recapTime: string;
  notificationCron: string;
  smtp: SMTPSettingsState;
}

export interface SSOSettingsState {
  enabled: boolean;
  provider: string;
  issuerUrl: string;
  clientId: string;
  clientSecret: string;
  redirectUrl: string;
}

export interface SettingsFormState {
  databaseUrl: string;
  clientOrigin: string;
  timezone: string;
  autoPruneImages: boolean;
  hideSupportButton: boolean;
  backupDestination: BackupDestinationSettings;
  notifications: NotificationSettingsState;
  sso: SSOSettingsState;
}
