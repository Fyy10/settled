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
	groups: {
		createAction: 'Create group',
		joinAction: 'Join with code',
		loading: 'Loading your groups',
		loadFailureTitle: 'Settled couldn’t load your groups.',
		loadFailureDescription: 'Check your connection and try again.',
		retry: 'Retry',
		emptyTitle: 'No groups yet',
		emptyDescription: 'Create a group for a trip or household, or join one with a code.',
		owner: 'Owner',
		updated: 'Updated',
		create: {
			title: 'Create group',
			description: 'Give this shared ledger a name everyone will recognize.',
			nameLabel: 'Group name',
			nameRequired: 'Enter a group name.',
			nameTooLong: 'Group name must not exceed 160 characters.',
			cancel: 'Cancel',
			submit: 'Create group',
			pending: 'Creating group',
			failure: 'Settled could not create the group. Check the name and try again.',
			success: 'Group created'
		},
		join: {
			title: 'Join a group',
			description: 'Enter the private group code shared with you.',
			codeLabel: 'Group code',
			codeRequired: 'Enter a group code.',
			cancel: 'Cancel',
			submit: 'Join group',
			pending: 'Joining group',
			invalidCode: 'That group code is not valid.',
			failure: 'Settled could not join the group. Check the code and try again.',
			success: 'Group joined'
		},
		workspace: {
			loading: 'Loading group',
			loadFailureTitle: 'Settled couldn’t load this group.',
			loadFailureDescription: 'Check your connection and try again.',
			hiddenTitle: 'This group isn’t available.',
			hiddenDescription: 'It may have been dissolved or you may no longer have access.',
			retry: 'Retry',
			backToGroups: 'Back to groups',
			groups: 'Groups',
			owner: 'Owner',
			addExpense: 'Add expense',
			recordPayment: 'Record payment',
			actionsLabel: 'Group accounting actions',
			openMenu: 'Open group menu',
			settings: 'Group settings',
			tabsLabel: 'Group views',
			balancesTab: 'Balances',
			activityTab: 'Activity',
			membersTab: 'Members',
			unknownMember: 'Unknown member',
			balances: {
				title: 'Balances',
				description: 'Current payment suggestions from the shared ledger.',
				loading: 'Loading balances',
				failureTitle: 'Settled couldn’t load balances.',
				failureDescription: 'Other group information is still available.',
				emptyTitle: 'All settled',
				emptyDescription: 'There are no current balances in this group.',
				shouldPay: 'should pay'
			},
			activity: {
				title: 'Activity',
				description: 'Expenses and payment records, grouped by date.',
				loadingExpenses: 'Loading expenses',
				loadingPayments: 'Loading payment records',
				expenseFailureTitle: 'Settled couldn’t load expenses.',
				paymentFailureTitle: 'Settled couldn’t load payment records.',
				partialFailureDescription: 'Loaded activity remains available below.',
				emptyTitle: 'No activity yet',
				emptyDescription: 'Add an expense or record a payment to start this ledger.',
				paid: 'paid',
				splitWith: 'Split with',
				person: 'person',
				people: 'people',
				recordedOutside: 'Recorded outside Settled'
			},
			members: {
				title: 'Members',
				description: 'People with access to this shared ledger.',
				emptyTitle: 'No members available',
				emptyDescription: 'Member information could not be shown.',
				joined: 'Joined',
				remove: 'Remove member',
				removeTitle: 'Remove member?',
				removeDescription: 'They will lose access to this group and its history.',
				removeCancel: 'Cancel',
				removeSubmit: 'Remove member',
				removePending: 'Removing member',
				removeConflict:
					'This member is used by a current expense or payment record. Update or delete those records before removing them.',
				removeFailure: 'Settled could not remove this member. Try again.',
				removeRefreshFailure:
					'The member was removed, but some group information could not be refreshed.',
				removed: 'Member removed'
			}
		},
		settings: {
			backToGroup: 'Back to group',
			title: 'Group settings',
			description: 'Manage this private ledger and who can access it.',
			redirecting: 'Returning to the group',
			redirectFailure: 'Settled could not return to the group automatically.',
			name: {
				title: 'Group name',
				description: 'Use a name everyone in the group will recognize.',
				label: 'Group name',
				submit: 'Save changes',
				pending: 'Saving changes',
				success: 'Changes saved',
				failure: 'Settled could not save the group name. Check it and try again.',
				refreshFailure:
					'Changes were saved, but some group information could not be refreshed.'
			},
			code: {
				title: 'Group code',
				description:
					'Anyone with this code can join the group. Share it only with people you trust.',
				loading: 'Loading group code',
				loadFailure: 'Settled could not load the group code.',
				retry: 'Retry',
				copy: 'Copy code',
				copied: 'Code copied',
				copyFailure: 'Copy failed. The code is selected so you can copy it manually.'
			},
			danger: {
				title: 'Danger zone',
				description: 'Dissolving hides the group and stops all new activity.',
				action: 'Dissolve group',
				dialogTitle: 'Dissolve this group?',
				dialogDescription:
					'The group will disappear from ordinary views and no new activity can be added.',
				confirmationLabel: 'Type the current group name to confirm',
				confirmationDescription: 'This confirmation is case-sensitive.',
				cancel: 'Cancel',
				submit: 'Dissolve group',
				pending: 'Dissolving group',
				success: 'Group dissolved',
				failure: 'Settled could not dissolve this group. Try again.',
				refreshFailure:
					'The group was dissolved, but the group list could not be refreshed before navigation.',
				navigationFailure:
					'The group was dissolved, but Settled could not open your groups.',
				continueToGroups: 'Continue to groups'
			}
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
