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
  KBListResponse,
  MyAssetsResponse,
  AdminMappingsResponse,
} from './types';

const BASE = '/plugins/com.ntas.glpi/api/v1';
const TIMEOUT_MS = 30000;
const MAX_RETRIES = 2;

// requestJSON is the single HTTP transport for every API call. It
// automatically attaches credentials (Mattermost session cookie for
// browser-based authentication), enforces a timeout, and retries
// GET/DELETE on transient network failures.
async function requestJSON<T>(url: string, options?: RequestInit): Promise<T> {
  const method = (options?.method || 'GET').toUpperCase();
  const isSafe = method === 'GET' || method === 'HEAD';

  const doFetch = async (attempt: number): Promise<T> => {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), TIMEOUT_MS);

    try {
      const res = await fetch(url, {
        ...options,
        headers: {
          'Content-Type': 'application/json',
          'X-Requested-With': 'XMLHttpRequest',
          ...options?.headers,
        },
        credentials: 'same-origin',
        signal: controller.signal,
      });

      if (res.status === 401 || res.status === 403) {
        throw new Error('not authenticated');
      }

      const body = await res.json().catch(() => null);
      if (!body || body.status === 'error') {
        throw new Error(body?.error || `request failed (${res.status})`);
      }

      return body.data as T;
    } catch (err: unknown) {
      const isAbort = err instanceof DOMException && err.name === 'AbortError';
      const isNetwork = err instanceof TypeError && attempt < MAX_RETRIES;
      if (isAbort) throw new Error('request timed out');
      if (isNetwork && isSafe) {
        await new Promise((r) => setTimeout(r, 200 * (attempt + 1)));
        return doFetch(attempt + 1);
      }
      throw err;
    } finally {
      clearTimeout(timer);
    }
  };

  return doFetch(0);
}

export const api = {
  getStatus(): Promise<StatusResponse> {
    return requestJSON<StatusResponse>(`${BASE}/status`);
  },

  getConfig(): Promise<ConfigResponse> {
    return requestJSON<ConfigResponse>(`${BASE}/config`);
  },

  getUser(): Promise<UserResponse> {
    return requestJSON<UserResponse>(`${BASE}/user`);
  },

  listTickets(type: 'my' | 'assigned' | 'all', search?: string, page = 1, perPage = 15, status?: number, sort?: number, order?: 'ASC' | 'DESC'): Promise<TicketListResponse> {
    const params = new URLSearchParams({ type, per_page: String(perPage), page: String(page) });
    if (search) params.set('search', search);
    if (status) params.set('status', String(status));
    if (sort) params.set('sort', String(sort));
    if (order) params.set('order', order);
    return requestJSON<TicketListResponse>(`${BASE}/tickets?${params}`);
  },

  createTicket(req: CreateTicketRequest): Promise<CreateTicketResponse> {
    return requestJSON<CreateTicketResponse>(`${BASE}/tickets`, {
      method: 'POST',
      body: JSON.stringify(req),
    });
  },

  getTicket(id: number): Promise<Ticket> {
    return requestJSON<Ticket>(`${BASE}/tickets/${id}`);
  },

  updateTicket(id: number, input: Record<string, unknown>): Promise<{ status: string }> {
    return requestJSON<{ status: string }>(`${BASE}/tickets/${id}`, {
      method: 'PUT',
      body: JSON.stringify(input),
    });
  },

  deleteTicket(id: number): Promise<{ status: string }> {
    return requestJSON<{ status: string }>(`${BASE}/tickets/${id}`, { method: 'DELETE' });
  },

  addFollowup(ticketId: number, content: string, isPrivate: boolean): Promise<{ status: string }> {
    return requestJSON<{ status: string }>(`${BASE}/tickets/${ticketId}/followup`, {
      method: 'POST',
      body: JSON.stringify({ content, is_private: isPrivate }),
    });
  },

  addSolution(ticketId: number, content: string): Promise<{ status: string }> {
    return requestJSON<{ status: string }>(`${BASE}/tickets/${ticketId}/solution`, {
      method: 'POST',
      body: JSON.stringify({ content }),
    });
  },

  getTimeline(ticketId: number, page = 1, perPage = 20): Promise<TimelinePage> {
    const params = new URLSearchParams({ page: String(page), per_page: String(perPage) });
    return requestJSON<TimelinePage>(`${BASE}/tickets/${ticketId}/timeline?${params}`);
  },

  listAssets(type: string, search?: string, page = 1, perPage = 15): Promise<AssetListResponse> {
    const params = new URLSearchParams({ type, page: String(page), per_page: String(perPage) });
    if (search) params.set('search', search);
    return requestJSON<AssetListResponse>(`${BASE}/assets?${params}`);
  },

  searchKnowledge(query: string, page = 1, perPage = 15, category?: number): Promise<KnowledgeResponse> {
    const params = new URLSearchParams({ q: query, page: String(page), per_page: String(perPage) });
    if (category) params.set('category', String(category));
    return requestJSON<KnowledgeResponse>(`${BASE}/knowledge?${params}`);
  },

  getKnowledgeCategories(): Promise<KnowledgeCategoryListResponse> {
    return requestJSON<KnowledgeCategoryListResponse>(`${BASE}/knowledge/categories`);
  },

  async attachFile(ticketId: number, file: File): Promise<{ document_id: number; filename: string }> {
    const form = new FormData();
    form.append('file', file);
    const res = await fetch(`${BASE}/tickets/${ticketId}/attach`, {
      method: 'POST',
      body: form,
      credentials: 'same-origin',
      headers: { 'X-Requested-With': 'XMLHttpRequest' },
    });
    if (res.status === 401 || res.status === 403) {
      throw new Error('not authenticated');
    }
    const body = await res.json().catch(() => null);
    if (body?.status === 'error') throw new Error(body.error || 'upload failed');
    return body.data;
  },

  // ---------- Extended v1.0 endpoints ----------

  getDashboard(): Promise<DashboardData> {
    return requestJSON<DashboardData>(`${BASE}/dashboard`);
  },

  getSystem(): Promise<SystemInfo> {
    return requestJSON<SystemInfo>(`${BASE}/system`);
  },

  getCategories(query?: string): Promise<CategoryListResponse> {
    const params = new URLSearchParams();
    if (query) params.set('q', query);
    return requestJSON<CategoryListResponse>(`${BASE}/categories?${params}`);
  },

  getKnowledgeArticle(id: number): Promise<KnowledgeArticle> {
    return requestJSON<KnowledgeArticle>(`${BASE}/knowledge/${id}`);
  },

  getAssetDetail(type: string, id: number): Promise<AssetDetail> {
    return requestJSON<AssetDetail>(`${BASE}/assets/${type}/${id}`);
  },

  listTicketDocuments(ticketId: number): Promise<{ documents: DocumentInfo[]; count: number }> {
    return requestJSON<{ documents: DocumentInfo[]; count: number }>(`${BASE}/tickets/${ticketId}/documents`);
  },

  getNotifications(): Promise<NotificationListResponse> {
    return requestJSON<NotificationListResponse>(`${BASE}/notifications`);
  },

  markNotificationRead(id: string): Promise<{ status: string }> {
    return requestJSON<{ status: string }>(`${BASE}/notifications/${id}/read`, { method: 'POST' });
  },

  dismissNotification(id: string): Promise<{ status: string }> {
    return requestJSON<{ status: string }>(`${BASE}/notifications/${id}/dismiss`, { method: 'POST' });
  },

  async downloadDocument(ticketId: number, docId: number, filename: string): Promise<void> {
    const res = await fetch(`${BASE}/tickets/${ticketId}/documents/${docId}`, {
      credentials: 'same-origin',
      headers: { 'X-Requested-With': 'XMLHttpRequest' },
    });
    if (res.status === 401 || res.status === 403) {
      throw new Error('not authenticated');
    }
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

  // ---- Identity / enterprise additions ----

  getMyAssets(): Promise<MyAssetsResponse> {
    return requestJSON<MyAssetsResponse>(`${BASE}/my-assets`);
  },

  getKBFavorites(): Promise<KBListResponse> {
    return requestJSON<KBListResponse>(`${BASE}/knowledge/favorites`);
  },

  getKBRecent(): Promise<KBListResponse> {
    return requestJSON<KBListResponse>(`${BASE}/knowledge/recent`);
  },

  setKBFavorite(id: number, subject: string, favorite: boolean): Promise<{ id: number; favorite: boolean }> {
    return requestJSON<{ id: number; favorite: boolean }>(`${BASE}/knowledge/${id}/favorite`, {
      method: favorite ? 'POST' : 'DELETE',
      body: JSON.stringify({ subject }),
    });
  },

  getAdminMappings(): Promise<AdminMappingsResponse> {
    return requestJSON<AdminMappingsResponse>(`${BASE}/admin/mappings`);
  },

  adminSyncUsers(): Promise<{ mapped: number; skipped: number; errors: number; synced_at: number }> {
    return requestJSON<{ mapped: number; skipped: number; errors: number; synced_at: number }>(`${BASE}/admin/sync`, {
      method: 'POST',
    });
  },

  adminProvisionUser(userId: string, profileId = 5, entityId = 0): Promise<{ mapping: unknown; glpi_user_id: number }> {
    return requestJSON<{ mapping: unknown; glpi_user_id: number }>(`${BASE}/admin/provision`, {
      method: 'POST',
      body: JSON.stringify({ user_id: userId, profile_id: profileId, entity_id: entityId }),
    });
  },

  adminClearCache(): Promise<{ cleared: boolean }> {
    return requestJSON<{ cleared: boolean }>(`${BASE}/admin/clear-cache`, { method: 'POST' });
  },
};
