import type { LayoutLoad } from './$types';

export const load: LayoutLoad = ({ params }) => ({
	groupId: params.groupId
});
