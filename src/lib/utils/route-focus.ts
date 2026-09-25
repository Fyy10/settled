export function shouldFocusRouteHeading(
	from: URL | null | undefined,
	to: URL
): boolean {
	return from != null && from.pathname !== to.pathname;
}
