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
  KnowledgeResponse,
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

  listTickets(type: 'my' | 'assigned' | 'all', search?: string, page = 1, perPage = 15): Promise<TicketListResponse> {
    const params = new URLSearchParams({ type, per_page: String(perPage), page: String(page) });
    if (search) params.set('search', search);
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

  listAssets(type: string, search?: string): Promise<AssetListResponse> {
    const params = new URLSearchParams({ type });
    if (search) params.set('search', search);
    return request<AssetListResponse>(`${BASE}/assets?${params}`);
  },

  searchKnowledge(query: string): Promise<KnowledgeResponse> {
    const params = new URLSearchParams({ q: query });
    return request<KnowledgeResponse>(`${BASE}/knowledge?${params}`);
  },

  async attachFile(ticketId: number, file: File): Promise<{ document_id: number; filename: string }> {
    const form = new FormData();
    form.append('file', file);
    const res = await fetch(`${BASE}/tickets/${ticketId}/attach`, { method: 'POST', body: form });
    const body = await res.json();
    if (body.status === 'error') throw new Error(body.error || 'upload failed');
    return body.data;
  },
};
