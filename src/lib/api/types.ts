export type ApiErrorBody = {
	error: {
		code: string;
		message: string;
		fields?: Record<string, string>;
	};
};

export type User = {
	id: string;
	email: string;
	displayName: string;
	createdAt: string;
	updatedAt: string;
};

export type GroupRole = 'owner' | 'member';

export type GroupSummary = {
	id: string;
	name: string;
	ownerUserId: string;
	memberCount: number;
	currentUserRole: GroupRole;
	createdAt: string;
	updatedAt: string;
};

export type GroupMember = {
	userId: string;
	email: string;
	displayName: string;
	role: GroupRole;
	joinedAt: string;
};

export type MemberSummary = {
	userId: string;
	displayName: string;
};

export type ExpenseSplit = {
	userId: string;
	amountCents: number;
};

export type Expense = {
	id: string;
	groupId: string;
	paidByUserId: string;
	description: string;
	amountCents: number;
	currency: 'USD';
	expenseDate: string;
	createdByUserId: string;
	splits: ExpenseSplit[];
	createdAt: string;
	updatedAt: string;
};

export type Repayment = {
	id: string;
	groupId: string;
	fromUserId: string;
	toUserId: string;
	amountCents: number;
	currency: 'USD';
	note: string | null;
	repaymentDate: string;
	createdByUserId: string;
	createdAt: string;
	updatedAt: string;
};

export type Settlement = {
	fromUserId: string;
	toUserId: string;
	amountCents: number;
	currency: 'USD';
};

export type HealthResponse = {
	status: 'ok' | 'unavailable';
};

export type CsrfResponse = {
	csrfToken: string;
};

export type RegisterInput = {
	email: string;
	password: string;
	displayName: string;
};

export type LoginCredentials = {
	email: string;
	password: string;
};

export type AuthResponse = {
	csrfToken: string;
	user: User;
};

export type CurrentUserResponse = {
	user: User;
};

export type GroupListResponse = {
	groups: GroupSummary[];
};

export type CreateGroupInput = {
	name: string;
};

export type JoinGroupInput = {
	joinCode: string;
};

export type RenameGroupInput = {
	name: string;
};

export type GroupResponse = {
	group: GroupSummary;
};

export type GroupDetailResponse = {
	group: GroupSummary;
	members: GroupMember[];
};

export type JoinCodeResponse = {
	joinCode: string;
};

export type ExpenseBaseInput = {
	paidByUserId: string;
	description: string;
	amountCents: number;
	expenseDate: string;
};

export type EqualExpenseInput = ExpenseBaseInput & {
	splitMode: 'equal';
	participantUserIds: string[];
};

export type ExactExpenseInput = ExpenseBaseInput & {
	splitMode: 'exact';
	splits: ExpenseSplit[];
};

export type PercentageExpenseInput = ExpenseBaseInput & {
	splitMode: 'percentage';
	percentageSplits: Array<{
		userId: string;
		percentageBasisPoints: number;
	}>;
};

export type ExpenseInput = EqualExpenseInput | ExactExpenseInput | PercentageExpenseInput;

export type ExpenseListResponse = {
	expenses: Expense[];
	members: MemberSummary[];
};

export type ExpenseResponse = {
	expense: Expense;
};

export type RepaymentBaseInput = {
	fromUserId: string;
	toUserId: string;
	amountCents: number;
	repaymentDate: string;
};

export type CreateRepaymentInput = RepaymentBaseInput & {
	note?: string | null;
};

export type ReplaceRepaymentInput = RepaymentBaseInput & {
	note: string | null;
};

export type RepaymentListResponse = {
	repayments: Repayment[];
	members: MemberSummary[];
};

export type RepaymentResponse = {
	repayment: Repayment;
};

export type SettlementListResponse = {
	settlements: Settlement[];
	members: MemberSummary[];
};
