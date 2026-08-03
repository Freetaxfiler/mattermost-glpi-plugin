// GLPI data types matching the server response format.

export interface TicketSummary {
  ID: number;
  Name: string;
  Status: number;
  Priority: number;
  Opened: string;
}

export interface Ticket {
  id: number;
  name: string;
  content: string;
  status: number;
  priority: number;
  urgency: number;
  impact: number;
  date: string;
  date_mod: string;
}

export interface CreateTicketRequest {
  subject: string;
  content: string;
  priority: number;
  urgency: number;
  category_id: number;
  type?: number;
}

export interface CreateTicketResponse {
  id: number;
}

export interface AssetSummary {
  ID: number;
  Name: string;
  Serial: string;
  ItemType?: string;
}

export interface KnowledgeSummary {
  ID: number;
  Subject: string;
}

export interface TimelineEvent {
  ID: number;
  Kind: string;
  Content: string;
  Date: string;
  AuthorID: number;
  Author: string;
  IsPrivate: boolean;
  Status: string;
}

export interface TimelinePage {
  Events: TimelineEvent[];
  Page: number;
  PerPage: number;
  Total: number;
  HasMore: boolean;
}

export interface StatusResponse {
  glpi_url: string;
  configured: boolean;
  glpi_version: string;
  glpi_online: boolean;
  plugin_version: string;
}

export interface ConfigResponse {
  glpi_url: string;
  default_entity: string;
  default_category: string;
  notification_channel_id: string;
  enable_debug: boolean;
}

export interface UserResponse {
  id: string;
  username: string;
  email: string;
  glpi_user_id: number;
  glpi_login: string;
  glpi_email: string;
  glpi_full_name: string;
  role: string;
  sync_status: string;
  is_system_admin: boolean;
}

// Knowledge base personalization (favorites / recently viewed).
export interface KBItem {
  id: number;
  subject: string;
  at: number;
}

export interface KBListResponse {
  items: KBItem[];
  count: number;
}

export interface MyAssetsResponse {
  assets: AssetSummary[];
  total: number;
  count: number;
  mapped: boolean;
}

// Admin identity-mapping page payload.
export interface AdminMappingRecord {
  mm_user_id: string;
  mm_username: string;
  mm_email: string;
  mm_display_name: string;
  glpi_user_id: number;
  glpi_login: string;
  glpi_full_name: string;
  glpi_email: string;
  profiles: string[];
  role: string;
  sync_status: string;
  last_sync: number;
}

export interface AdminUnmappedUser {
  user_id: string;
  username: string;
  email: string;
}

export interface AdminMappingsResponse {
  mappings: AdminMappingRecord[];
  unmapped: AdminUnmappedUser[];
  duplicate_emails: AdminUnmappedUser[][];
  mm_user_count: number;
  mapping_enabled: boolean;
}

export interface TicketListResponse {
  tickets: TicketSummary[];
  total: number;
  count: number;
}

export interface AssetListResponse {
  assets: AssetSummary[];
  total: number;
  count: number;
}

export interface KnowledgeResponse {
  articles: KnowledgeSummary[];
  total: number;
  count: number;
}

export type ViewName =
  | 'dashboard'
  | 'create-ticket'
  | 'my-tickets'
  | 'assigned-tickets'
  | 'search'
  | 'ticket-details'
  | 'assets'
  | 'my-assets'
  | 'knowledge-base'
  | 'notifications'
  | 'settings'
  | 'admin';

// ITIL categories for the ticket category picker.
export interface CategorySummary {
  ID: number;
  Name: string;
}

export interface CategoryListResponse {
  categories: CategorySummary[];
  total: number;
  count: number;
}

// Full knowledge base article (body is GLPI rich-text HTML).
export interface KnowledgeArticle {
  id: number;
  subject: string;
  content: string;
  category: string;
  date: string;
  date_mod: string;
}

// Knowledge base category for the KB filter.
export interface KnowbaseCategorySummary {
  ID: number;
  Name: string;
}

export interface KnowledgeCategoryListResponse {
  categories: KnowbaseCategorySummary[];
  total: number;
  count: number;
}

// Normalized single-asset detail view.
export interface AssetDetail {
  id: number;
  itemtype: string;
  name: string;
  serial: string;
  otherserial: string;
  manufacturer: string;
  model: string;
  location: string;
  user: string;
  tech_user: string;
  state: string;
  warranty_date: string;
  notes: string;
}

// Document attached to a ticket.
export interface DocumentInfo {
  id: number;
  name: string;
  filename: string;
  mime_type: string;
  size: number;
}

// Aggregated dashboard statistics.
export interface DashboardData {
  open: number;
  assigned: number;
  resolved: number;
  pending: number;
  closed: number;
  critical: number;
  overdue: number;
  recent: TicketSummary[];
}

// Persisted notification shown in the notification center.
export interface AppNotification {
  id: string;
  type: string;
  ticket_id: number;
  title: string;
  status: string;
  url: string;
  created_at: number;
}

export interface NotificationListResponse {
  notifications: AppNotification[];
  unread: number;
}

// Plugin runtime information for the Settings page.
export interface SystemInfo {
  plugin_version: string;
  mattermost_version: string;
  glpi_connected: boolean;
  debug: boolean;
  mapping: {
    enabled: boolean;
    mapped: number;
    last_sync: number;
  };
  retry_queue: {
    workers: number;
    max_attempts: number;
    backoff_base: number;
    pending: number;
  };
  webhook_configured: boolean;
}
