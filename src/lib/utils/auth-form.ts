import { copy } from '$lib/copy/en';

export type LoginFormValues = {
	email: string;
	password: string;
};

export type LoginFieldErrors = Partial<Record<keyof LoginFormValues, string>>;

export type RegistrationFormValues = {
	displayName: string;
	email: string;
	password: string;
};

export type RegistrationFieldErrors = Partial<
	Record<keyof RegistrationFormValues, string>
>;

export function validateLoginForm(values: Readonly<LoginFormValues>): LoginFieldErrors {
	const errors: LoginFieldErrors = {};
	const email = values.email.trim();

	if (email === '') {
		errors.email = copy.auth.login.emailRequired;
	} else if (!isObviousEmail(email)) {
		errors.email = copy.auth.login.emailInvalid;
	}
	if (values.password === '') {
		errors.password = copy.auth.login.passwordRequired;
	}

	return errors;
}

export function validateRegistrationForm(
	values: Readonly<RegistrationFormValues>
): RegistrationFieldErrors {
	const errors: RegistrationFieldErrors = {};
	const displayName = values.displayName.trim();
	const email = values.email.trim();
	const passwordBytes = new TextEncoder().encode(values.password).length;

	if (displayName === '') {
		errors.displayName = copy.auth.register.displayNameRequired;
	} else if ([...displayName].length > 120) {
		errors.displayName = copy.auth.register.displayNameTooLong;
	}
	if (email === '') {
		errors.email = copy.auth.register.emailRequired;
	} else if (!isObviousEmail(email)) {
		errors.email = copy.auth.register.emailInvalid;
	}
	if (passwordBytes < 8 || passwordBytes > 128) {
		errors.password = copy.auth.register.passwordInvalid;
	}

	return errors;
}

function isObviousEmail(value: string): boolean {
	return /^[^\s@:]+@[^\s@:]+$/.test(value);
}
