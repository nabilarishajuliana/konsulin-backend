package helper

type PermissionSet struct {
	matrix map[string]map[string]bool
}

func NewPermissionSet() *PermissionSet {
	return &PermissionSet{matrix: defaultMatrix()}
}

func (p *PermissionSet) Has(role, permission string) bool {
	perms, ok := p.matrix[role]
	if !ok {
		return false
	}
	return perms[permission]
}

func (p *PermissionSet) PermissionsOf(role string) []string {
	out := make([]string, 0)
	for perm, allowed := range p.matrix[role] {
		if allowed {
			out = append(out, perm)
		}
	}
	return out
}

func set(perms ...string) map[string]bool {
	m := make(map[string]bool, len(perms))
	for _, perm := range perms {
		m[perm] = true
	}
	return m
}

func defaultMatrix() map[string]map[string]bool {
	return map[string]map[string]bool{
		"mahasiswa": set(
			"slot:list", "slot:read",
			"booking:create", "booking:confirm", "booking:cancel", "booking:swap", "booking:read_own",
			"note:write", "note:read",
		),
		"dosen": set(
			"slot:list", "slot:read", "slot:create", "slot:update", "slot:delete", "slot:read_bookers",
			"booking:attendance", "note:write", "note:read",
		),
		"admin": set(
			"slot:list", "slot:read", "slot:create", "slot:update", "slot:delete", "slot:read_bookers",
			"booking:cancel", "booking:attendance",
			"note:read", "user:manage", "queue:monitor", "recap:view",
		),
	}
}
