export function shouldFocusRouteHeading(from: URL | undefined, to: URL): boolean {
	return from !== undefined && from.pathname !== to.pathname;
}
