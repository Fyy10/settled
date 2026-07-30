export type ApiErrorDetails = {
	status: number;
	code: string;
	message: string;
	fields: Record<string, string>;
	requestId?: string;
};

export type ApiErrorSource = 'http' | 'network' | 'invalid-response';

export type ApiErrorCategory = 'unauthorized' | 'csrf' | 'network' | 'api' | 'unknown';

export class ApiError extends Error {
	readonly status: number;
	readonly code: string;
	readonly fields: Record<string, string>;
	readonly requestId?: string;
	readonly source: ApiErrorSource;

	constructor(
		details: ApiErrorDetails,
		source: ApiErrorSource = 'http',
		options?: { cause?: unknown }
	) {
		super(details.message, options);
		this.name = 'ApiError';
		this.status = details.status;
		this.code = details.code;
		this.fields = { ...details.fields };
		this.requestId = details.requestId;
		this.source = source;
	}
}

export function isUnauthorizedApiError(error: unknown): error is ApiError {
	return error instanceof ApiError && error.status === 401;
}

export function isCsrfApiError(error: unknown): error is ApiError {
	return (
		error instanceof ApiError &&
		error.status === 403 &&
		(error.code === 'csrf_required' || error.code === 'csrf_invalid')
	);
}

export function isNetworkApiError(error: unknown): error is ApiError {
	return error instanceof ApiError && error.source === 'network';
}

export function classifyApiError(error: unknown): ApiErrorCategory {
	if (isUnauthorizedApiError(error)) {
		return 'unauthorized';
	}
	if (isCsrfApiError(error)) {
		return 'csrf';
	}
	if (isNetworkApiError(error)) {
		return 'network';
	}
	if (error instanceof ApiError && error.source === 'http') {
		return 'api';
	}

	return 'unknown';
}

export type MappedApiFieldErrors<FormField extends string> = {
	fieldErrors: Partial<Record<FormField, string>>;
	unknownFieldErrors: Record<string, string>;
};

export function mapApiFieldErrors<FormField extends string>(
	fields: Readonly<Record<string, string>>,
	fieldMap: Readonly<Record<string, FormField>>
): MappedApiFieldErrors<FormField> {
	const fieldErrors: Partial<Record<FormField, string>> = {};
	const unknownFieldErrors: Record<string, string> = {};

	for (const [apiField, message] of Object.entries(fields)) {
		const formField = fieldMap[apiField];
		if (formField === undefined) {
			unknownFieldErrors[apiField] = message;
			continue;
		}

		fieldErrors[formField] ??= message;
	}

	return { fieldErrors, unknownFieldErrors };
}

export const apiFieldMap = {
	displayName: 'displayName',
	email: 'email',
	password: 'password',
	name: 'name',
	joinCode: 'joinCode',
	description: 'description',
	amountCents: 'amount',
	paidByUserId: 'payer',
	expenseDate: 'date',
	participantUserIds: 'splits',
	splits: 'splits',
	percentageSplits: 'splits',
	fromUserId: 'sender',
	toUserId: 'recipient',
	repaymentDate: 'date',
	note: 'note'
} as const;

export function invalidResponseError(
	status: number,
	requestId?: string,
	message = 'The server returned an invalid response.'
): ApiError {
	return new ApiError(
		{
			status,
			code: 'invalid_response',
			message,
			fields: {},
			requestId
		},
		'invalid-response'
	);
}

export function networkError(cause: unknown): ApiError {
	return new ApiError(
		{
			status: 0,
			code: 'network_error',
			message: "Settled can't reach the server.",
			fields: {}
		},
		'network',
		{ cause }
	);
}
