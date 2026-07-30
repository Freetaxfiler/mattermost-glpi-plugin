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
}

export interface CreateTicketResponse {
  id: number;
}

export interface AssetSummary {
  ID: number;
  Name: string;
  Serial: string;
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
  is_system_admin: boolean;
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
  | 'knowledge-base'
  | 'notifications'
  | 'settings';
