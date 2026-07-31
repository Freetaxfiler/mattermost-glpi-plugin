import type {
  StatusResponse,
  ConfigResponse,
  UserResponse,
  TicketListResponse,
  Ticket,
  CreateTicketRequest,
  CreateTicketResponse,
  TimelinePage,
  AssetListResponse,
  AssetDetail,
  KnowledgeResponse,
  KnowledgeArticle,
  KnowledgeCategoryListResponse,
  CategoryListResponse,
  DashboardData,
  DocumentInfo,
  NotificationListResponse,
  SystemInfo,
} from './types';

const BASE = '/plugins/com.ntas.glpi/api/v1';

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    ...options,
  });
  const body = await res.json();
  if (body.status === 'error') {
    throw new Error(body.error || 'request failed');
  }
  return body.data as T;
}

export const api = {
  getStatus(): Promise<StatusResponse> {
    return request<StatusResponse>(`${BASE}/status`);
  },

  getConfig(): Promise<ConfigResponse> {
    return request<ConfigResponse>(`${BASE}/config`);
  },

  getUser(): Promise<UserResponse> {
    return request<UserResponse>(`${BASE}/user`);
  },

  listTickets(type: 'my' | 'assigned' | 'all', search?: string, page = 1, perPage = 15, status?: number, sort?: number, order?: 'ASC' | 'DESC'): Promise<TicketListResponse> {
    const params = new URLSearchParams({ type, per_page: String(perPage), page: String(page) });
    if (search) params.set('search', search);
    if (status) params.set('status', String(status));
    if (sort) params.set('sort', String(sort));
    if (order) params.set('order', order);
    return request<TicketListResponse>(`${BASE}/tickets?${params}`);
  },

  createTicket(req: CreateTicketRequest): Promise<CreateTicketResponse> {
    return request<CreateTicketResponse>(`${BASE}/tickets`, {
      method: 'POST',
      body: JSON.stringify(req),
    });
  },

  getTicket(id: number): Promise<Ticket> {
    return request<Ticket>(`${BASE}/tickets/${id}`);
  },

  updateTicket(id: number, input: Record<string, unknown>): Promise<{ status: string }> {
    return request<{ status: string }>(`${BASE}/tickets/${id}`, {
      method: 'PUT',
      body: JSON.stringify(input),
    });
  },

  deleteTicket(id: number): Promise<{ status: string }> {
    return request<{ status: string }>(`${BASE}/tickets/${id}`, { method: 'DELETE' });
  },

  addFollowup(ticketId: number, content: string, isPrivate: boolean): Promise<{ status: string }> {
    return request<{ status: string }>(`${BASE}/tickets/${ticketId}/followup`, {
      method: 'POST',
      body: JSON.stringify({ content, is_private: isPrivate }),
    });
  },

  addSolution(ticketId: number, content: string): Promise<{ status: string }> {
    return request<{ status: string }>(`${BASE}/tickets/${ticketId}/solution`, {
      method: 'POST',
      body: JSON.stringify({ content }),
    });
  },

  getTimeline(ticketId: number, page = 1, perPage = 20): Promise<TimelinePage> {
    const params = new URLSearchParams({ page: String(page), per_page: String(perPage) });
    return request<TimelinePage>(`${BASE}/tickets/${ticketId}/timeline?${params}`);
  },

  listAssets(type: string, search?: string, page = 1, perPage = 15): Promise<AssetListResponse> {
    const params = new URLSearchParams({ type, page: String(page), per_page: String(perPage) });
    if (search) params.set('search', search);
    return request<AssetListResponse>(`${BASE}/assets?${params}`);
  },

  searchKnowledge(query: string, page = 1, perPage = 15, category?: number): Promise<KnowledgeResponse> {
    const params = new URLSearchParams({ q: query, page: String(page), per_page: String(perPage) });
    if (category) params.set('category', String(category));
    return request<KnowledgeResponse>(`${BASE}/knowledge?${params}`);
  },

  getKnowledgeCategories(): Promise<KnowledgeCategoryListResponse> {
    return request<KnowledgeCategoryListResponse>(`${BASE}/knowledge/categories`);
  },

  async attachFile(ticketId: number, file: File): Promise<{ document_id: number; filename: string }> {
    const form = new FormData();
    form.append('file', file);
    const res = await fetch(`${BASE}/tickets/${ticketId}/attach`, { method: 'POST', body: form });
    const body = await res.json();
    if (body.status === 'error') throw new Error(body.error || 'upload failed');
    return body.data;
  },

  // ---------- Extended v1.0 endpoints ----------

  getDashboard(): Promise<DashboardData> {
    return request<DashboardData>(`${BASE}/dashboard`);
  },

  getSystem(): Promise<SystemInfo> {
    return request<SystemInfo>(`${BASE}/system`);
  },

  getCategories(query?: string): Promise<CategoryListResponse> {
    const params = new URLSearchParams();
    if (query) params.set('q', query);
    return request<CategoryListResponse>(`${BASE}/categories?${params}`);
  },

  getKnowledgeArticle(id: number): Promise<KnowledgeArticle> {
    return request<KnowledgeArticle>(`${BASE}/knowledge/${id}`);
  },

  getAssetDetail(type: string, id: number): Promise<AssetDetail> {
    return request<AssetDetail>(`${BASE}/assets/${type}/${id}`);
  },

  listTicketDocuments(ticketId: number): Promise<{ documents: DocumentInfo[]; count: number }> {
    return request<{ documents: DocumentInfo[]; count: number }>(`${BASE}/tickets/${ticketId}/documents`);
  },

  getNotifications(): Promise<NotificationListResponse> {
    return request<NotificationListResponse>(`${BASE}/notifications`);
  },

  markNotificationRead(id: string): Promise<{ status: string }> {
    return request<{ status: string }>(`${BASE}/notifications/${id}/read`, { method: 'POST' });
  },

  dismissNotification(id: string): Promise<{ status: string }> {
    return request<{ status: string }>(`${BASE}/notifications/${id}/dismiss`, { method: 'POST' });
  },

  async downloadDocument(ticketId: number, docId: number, filename: string): Promise<void> {
    const res = await fetch(`${BASE}/tickets/${ticketId}/documents/${docId}`);
    if (!res.ok) {
      const body = await res.json().catch(() => null);
      throw new Error((body && body.error) || 'download failed');
    }
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename || `document-${docId}`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  },
};
