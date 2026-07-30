export type ResolvedSession = 'anonymous' | 'authenticated';

export function rootRedirectTarget(session: ResolvedSession): '/groups' | '/login' {
	return session === 'authenticated' ? '/groups' : '/login';
}
