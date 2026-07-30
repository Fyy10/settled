export const copy = {
	appName: 'Settled',
	tagline: 'Shared bills, made clear.',
	navigation: {
		skipToContent: 'Skip to content',
		backToSettled: 'Back to Settled',
		brandHome: 'Settled home',
		groupsHome: 'Settled groups'
	},
	loading: {
		session: 'Checking your session',
		route: 'Loading the next page'
	},
	auth: {
		serviceUnavailableTitle: 'Settled can’t reach the server.',
		serviceUnavailableDescription: 'Check your connection and try again.',
		retry: 'Retry',
		checkingTitle: 'Checking your session',
		checkingDescription: 'Settled is finding your private ledger.',
		login: {
			title: 'Log in',
			description: 'Return to your groups and shared bills.',
			emailLabel: 'Email',
			passwordLabel: 'Password',
			submit: 'Log in',
			pending: 'Logging in',
			showPassword: 'Show password',
			hidePassword: 'Hide password',
			emailRequired: 'Enter your email address.',
			emailInvalid: 'Enter a valid email address.',
			passwordRequired: 'Enter your password.',
			invalidCredentials: 'Email or password is incorrect.',
			sessionExpired: 'Your session expired. Log in again to continue.',
			failure: 'Settled could not log you in. Check the form and try again.',
			newAccountPrompt: 'New to Settled?',
			newAccountAction: 'Create account'
		},
		register: {
			title: 'Create account',
			description: 'Start a private ledger for the people you share costs with.',
			displayNameLabel: 'Display name',
			emailLabel: 'Email',
			passwordLabel: 'Password',
			passwordGuidance: 'Use 8–128 characters.',
			submit: 'Create account',
			pending: 'Creating account',
			showPassword: 'Show password',
			hidePassword: 'Hide password',
			displayNameRequired: 'Enter your display name.',
			displayNameTooLong: 'Display name must not exceed 120 characters.',
			emailRequired: 'Enter your email address.',
			emailInvalid: 'Enter a valid email address.',
			passwordInvalid: 'Use 8–128 characters.',
			emailConflict: 'An account already uses this email.',
			failure: 'Settled could not create your account. Check the form and try again.',
			existingAccountPrompt: 'Already have an account?',
			existingAccountAction: 'Log in'
		},
		accountMenu: {
			open: 'Open account menu for',
			label: 'Account',
			logout: 'Log out',
			pending: 'Signing out',
			failure: 'Settled could not sign you out. Your current page is still open. Try again.'
		}
	},
	errors: {
		documentTitle: 'Error',
		unexpectedTitle: 'Something went wrong',
		unexpectedSummary: 'This page could not be shown.',
		unexpectedDescription: 'Try again. If the problem continues, return to Settled.',
		retry: 'Try again'
	},
	expenseDraft: {
		amountRequired: 'Enter an amount.',
		amountInvalid: 'Enter a dollar amount with up to two decimal places.',
		amountPositive: 'Amount must be greater than $0.00.',
		amountTooLarge: 'Amount is too large.',
		descriptionRequired: 'Enter a description.',
		descriptionTooLong: 'Description must not exceed 240 characters.',
		payerRequired: 'Choose who paid.',
		dateInvalid: 'Enter a valid date in YYYY-MM-DD format.',
		participantsRequired: 'Choose at least one participant.',
		participantInvalid: 'Each participant must have a user ID.',
		participantDuplicate: 'Each participant can appear only once.',
		equalZeroShare: 'The expense must include at least one cent per participant.',
		exactShareRequired: 'Enter an amount for each participant.',
		exactShareInvalid: 'Enter each share as a dollar amount with up to two decimal places.',
		exactSharePositive: 'Each exact share must be at least $0.01.',
		exactShareTooLarge: 'An exact share is too large.',
		exactTotalInvalid: 'Exact shares must add up to the expense amount.',
		percentageRequired: 'Enter a percentage for each participant.',
		percentageInvalid: 'Enter each percentage with up to two decimal places.',
		percentagePositive: 'Each percentage must be greater than 0%.',
		percentageTooLarge: 'A percentage cannot exceed 100%.',
		percentageTotalInvalid: 'Percentages must add up to 100%.',
		percentageZeroShare: 'Each percentage must produce at least a one-cent share.'
	},
	routes: {
		root: {
			title: 'Opening your ledger',
			description: 'Settled is checking where to take you.'
		},
		login: {
			documentTitle: 'Log in',
			title: 'Log in',
			description: 'Return to your groups and shared bills.',
			loadingLabel: 'Loading login'
		},
		register: {
			documentTitle: 'Create account',
			title: 'Create account',
			description: 'Start a private ledger for the people you share costs with.',
			loadingLabel: 'Loading account creation'
		},
		groups: {
			documentTitle: 'Groups',
			title: 'Your groups',
			description: 'Your private shared ledgers live here.',
			loadingLabel: 'Loading groups'
		},
		group: {
			documentTitle: 'Group',
			title: 'Group',
			description: 'Balances, activity, and members share one clear workspace.',
			loadingLabel: 'Loading group'
		},
		addExpense: {
			documentTitle: 'Add expense',
			title: 'Add expense',
			description: 'Record what was paid and how it should be split.',
			loadingLabel: 'Loading expense form'
		},
		editExpense: {
			documentTitle: 'Edit expense',
			title: 'Edit expense',
			description: 'Review the saved amounts before making a change.',
			loadingLabel: 'Loading expense'
		},
		recordPayment: {
			documentTitle: 'Record payment',
			title: 'Record payment',
			description: 'Record money paid outside Settled.',
			loadingLabel: 'Loading payment form'
		},
		editPayment: {
			documentTitle: 'Edit payment record',
			title: 'Edit payment record',
			description: 'Review the off-app payment record before making a change.',
			loadingLabel: 'Loading payment record'
		},
		settings: {
			documentTitle: 'Settings',
			title: 'Group settings',
			description: 'Manage the group name, code, members, and lifecycle.',
			loadingLabel: 'Loading group settings'
		}
	}
} as const;
