// Role helpers mirroring server/identity role ranks. Roles descend from
// administrator (most privileged) to employee (least).
export const ROLE_RANK: Record<string, number> = {
  employee: 1,
  technician: 2,
  supervisor: 3,
  manager: 4,
  administrator: 5,
};

export function isTechnicianOrHigher(role?: string): boolean {
  return (ROLE_RANK[role || 'employee'] || 1) >= 2;
}

export function isAdminOrManager(role?: string): boolean {
  return (ROLE_RANK[role || 'employee'] || 1) >= 4;
}
