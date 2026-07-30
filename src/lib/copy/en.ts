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
	errors: {
		documentTitle: 'Error',
		unexpectedTitle: 'Something went wrong',
		unexpectedSummary: 'This page could not be shown.',
		unexpectedDescription: 'Try again. If the problem continues, return to Settled.',
		retry: 'Try again'
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
