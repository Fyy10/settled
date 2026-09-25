import { copy } from '$lib/copy/en';

export type CreateGroupFieldErrors = {
	name?: string;
};

export type JoinGroupFieldErrors = {
	joinCode?: string;
};

const updatedDateFormatter = new Intl.DateTimeFormat('en-US', {
	year: 'numeric',
	month: 'short',
	day: 'numeric'
});

export function validateCreateGroupName(value: string): CreateGroupFieldErrors {
	const name = value.trim();
	if (name === '') {
		return { name: copy.groups.create.nameRequired };
	}
	if ([...name].length > 160) {
		return { name: copy.groups.create.nameTooLong };
	}

	return {};
}

export function normalizeJoinCode(value: string): string {
	return value.trim().toUpperCase();
}

export function validateJoinCode(value: string): JoinGroupFieldErrors {
	if (normalizeJoinCode(value) === '') {
		return { joinCode: copy.groups.join.codeRequired };
	}

	return {};
}

export function formatMemberCount(memberCount: number): string {
	return `${memberCount} ${memberCount === 1 ? 'member' : 'members'}`;
}

export function formatGroupUpdatedAt(value: string): string {
	const timestamp = new Date(value);
	if (Number.isNaN(timestamp.getTime())) {
		return value;
	}

	return updatedDateFormatter.format(timestamp);
}
