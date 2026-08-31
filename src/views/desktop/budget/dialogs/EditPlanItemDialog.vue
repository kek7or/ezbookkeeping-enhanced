<template>
    <v-dialog width="700" v-model="showState">
        <v-card class="pa-sm-1 pa-md-2">
            <template #title>
                <h4 class="text-h4 text-wrap">{{ dialogTitle }}</h4>
            </template>
            <v-card-text>
                <p class="text-body-2 text-medium-emphasis mb-4">
                    {{ wished ? tt('Something you want but have not decided on. It belongs to no month and counts towards nothing until you put it in one.') : tt('Something expected this month that no schedule would produce. Nothing is written to the ledger.') }}
                </p>
                <v-row>
                    <!-- a wishlist is things somebody wants to buy, so there is no income to
                         choose and nothing to ask about -->
                    <v-col cols="12" md="6" v-if="!wished">
                        <!-- toggle-buttons is what the rest of the app puts on a toggle that
                             holds words: the theme sizes these buttons as squares, on the
                             assumption that what is in one is an icon. -->
                        <v-btn-toggle class="toggle-buttons budget-plan-type-toggle w-100"
                                      density="comfortable" variant="outlined" color="primary" divided mandatory
                                      :disabled="submitting" v-model="type">
                            <v-btn :value="TransactionType.Expense">{{ tt('Expense') }}</v-btn>
                            <v-btn :value="TransactionType.Income">{{ tt('Income') }}</v-btn>
                        </v-btn-toggle>
                    </v-col>
                    <v-col cols="12" :md="wished ? 12 : 6">
                        <amount-input :currency="selectedAccountCurrency"
                                      :show-currency="true"
                                      :persistent-placeholder="true"
                                      :disabled="submitting"
                                      :label="tt('Amount')"
                                      :enable-formula="true"
                                      v-model="amount"/>
                    </v-col>
                    <v-col cols="12">
                        <v-text-field type="text" persistent-placeholder
                                      autofocus
                                      :disabled="submitting"
                                      :label="tt('Name')"
                                      :placeholder="tt('What is expected')"
                                      v-model="name"/>
                    </v-col>
                    <v-col cols="12" md="6">
                        <two-column-select primary-key-field="id" primary-value-field="id" primary-title-field="name"
                                           primary-icon-field="icon" primary-icon-type="category" primary-color-field="color"
                                           primary-hidden-field="hidden" primary-sub-items-field="subCategories"
                                           secondary-key-field="id" secondary-value-field="id" secondary-title-field="name"
                                           secondary-icon-field="icon" secondary-icon-type="category" secondary-color-field="color"
                                           secondary-hidden-field="hidden"
                                           :disabled="submitting"
                                           :enable-filter="true" :filter-placeholder="tt('Find category')" :filter-no-items-text="tt('No available category')"
                                           :label="tt('Category')" :placeholder="wished ? tt('Optional') : tt('Category')"
                                           :items="availableCategories"
                                           v-model="categoryId">
                        </two-column-select>
                    </v-col>
                    <v-col cols="12" md="6">
                        <two-column-select primary-key-field="id" primary-value-field="category"
                                           primary-title-field="name" primary-footer-field="displayBalance"
                                           primary-icon-field="icon" primary-icon-type="account"
                                           primary-sub-items-field="accounts"
                                           secondary-key-field="id" secondary-value-field="id"
                                           secondary-title-field="name" secondary-footer-field="displayBalance"
                                           secondary-icon-field="icon" secondary-icon-type="account" secondary-color-field="color"
                                           :disabled="submitting"
                                           :enable-filter="true" :filter-placeholder="tt('Find account')" :filter-no-items-text="tt('No available account')"
                                           :label="tt('Account')" :placeholder="wished ? tt('Optional') : tt('Account')"
                                           :items="allVisibleCategorizedAccounts"
                                           v-model="accountId">
                        </two-column-select>
                    </v-col>
                    <v-col cols="12">
                        <v-text-field type="text" persistent-placeholder
                                      :disabled="submitting"
                                      :label="tt('Description')"
                                      v-model="comment"/>
                    </v-col>
                </v-row>
            </v-card-text>
            <v-card-text>
                <div class="w-100 d-flex justify-center gap-4">
                    <v-btn color="primary" :disabled="!isInputValid || submitting" @click="save">
                        {{ tt('Save') }}
                        <v-progress-circular indeterminate size="22" class="ms-2" v-if="submitting"></v-progress-circular>
                    </v-btn>
                    <v-btn color="secondary" variant="tonal" :disabled="submitting" @click="cancel">
                        {{ tt('Cancel') }}
                    </v-btn>
                </div>
            </v-card-text>
        </v-card>

        <snack-bar ref="snackbar" />
    </v-dialog>
</template>

<script setup lang="ts">
import AmountInput from '@/components/desktop/AmountInput.vue';
import TwoColumnSelect from '@/components/desktop/TwoColumnSelect.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';

import { ref, computed, watch, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useSettingsStore } from '@/stores/setting.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { useBudgetPlanStore } from '@/stores/budgetPlan.ts';

import { CategoryType } from '@/core/category.ts';
import { TransactionType } from '@/core/transaction.ts';
import { BudgetPlanItem } from '@/models/budget_plan.ts';
import type { TransactionCategory } from '@/models/transaction_category.ts';
import type { CategorizedAccountWithDisplayBalance } from '@/models/account.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt, getCategorizedAccountsWithDisplayBalance } = useI18n();

const settingsStore = useSettingsStore();
const accountsStore = useAccountsStore();
const transactionCategoriesStore = useTransactionCategoriesStore();
const budgetPlanStore = useBudgetPlanStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

const showState = ref<boolean>(false);
const submitting = ref<boolean>(false);
const itemId = ref<string>('');
const itemYear = ref<number>(0);
const itemMonth = ref<number>(0);
const wished = ref<boolean>(false);
const type = ref<number>(TransactionType.Expense);
const categoryId = ref<string>('');
const accountId = ref<string>('');
const amount = ref<number>(0);
const name = ref<string>('');
const comment = ref<string>('');

let resolveFunc: ((item: BudgetPlanItem) => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const isNew = computed<boolean>(() => !itemId.value);

const dialogTitle = computed<string>(() => {
    if (wished.value) {
        return isNew.value ? tt('Add to Wishlist') : tt('Edit Wish');
    }

    return isNew.value ? tt('Plan Something') : tt('Edit Planned Item');
});

// The two selects take a bare list of records rather than the models the stores hold, so both are
// widened once here instead of at every binding.
const allVisibleCategorizedAccounts = computed<Record<string, unknown>[]>(() => {
    const accounts: CategorizedAccountWithDisplayBalance[] = getCategorizedAccountsWithDisplayBalance(accountsStore.allVisiblePlainAccounts, settingsStore.appSettings.showAccountBalance, settingsStore.appSettings.accountCategoryOrders);
    return accounts as unknown as Record<string, unknown>[];
});

// The category list follows the type, because an expense planned under an income category would
// make the plan disagree with the ledger it is going to be compared against.
const availableCategories = computed<Record<string, unknown>[]>(() => {
    const categoryType = type.value === TransactionType.Income ? CategoryType.Income : CategoryType.Expense;
    const categories: TransactionCategory[] = transactionCategoriesStore.allTransactionCategories[categoryType] || [];
    return categories as unknown as Record<string, unknown>[];
});

const selectedAccountCurrency = computed<string>(() => accountsStore.allAccountsMap[accountId.value]?.currency ?? budgetPlanStore.defaultCurrency);

// Switching the type drops a category that belongs to the other one. Left alone it would sit in
// the value while the select showed nothing chosen, and save as an income item filed under an
// expense category - which is the disagreement availableCategories exists to prevent. A category
// that survives the switch, which is none of them today, would be left where it is.
watch(type, () => {
    if (!categoryId.value) {
        return;
    }

    const stillAvailable = availableCategories.value.some(category => category['id'] === categoryId.value
        || ((category['subCategories'] as Record<string, unknown>[] | undefined) || []).some(subCategory => subCategory['id'] === categoryId.value));

    if (!stillAvailable) {
        categoryId.value = '';
    }
});

const isInputValid = computed<boolean>(() => {
    if (!name.value.trim() || amount.value === 0) {
        return false;
    }

    // a planned item is compared against the ledger and has to name what the ledger names; a wish is
    // compared against nothing yet, so it may name neither
    return wished.value || (!!categoryId.value && !!accountId.value);
});

function open(item: BudgetPlanItem): Promise<BudgetPlanItem> {
    showState.value = true;
    submitting.value = false;
    itemId.value = item.id;
    itemYear.value = item.year;
    itemMonth.value = item.month;
    wished.value = item.wished;
    type.value = item.type;
    categoryId.value = item.categoryId;
    accountId.value = item.accountId;
    amount.value = item.amount;
    name.value = item.name;
    comment.value = item.comment;

    return new Promise((resolve, reject) => {
        resolveFunc = resolve;
        rejectFunc = reject;
    });
}

function save(): void {
    if (!isInputValid.value) {
        return;
    }

    submitting.value = true;

    const item = BudgetPlanItem.of({
        id: itemId.value,
        year: itemYear.value,
        month: itemMonth.value,
        wished: wished.value,
        type: type.value,
        categoryId: categoryId.value,
        accountId: accountId.value,
        amount: amount.value,
        name: name.value.trim(),
        comment: comment.value,
        displayOrder: 0
    });

    const saving = wished.value
        ? budgetPlanStore.saveBudgetPlanWish({ wish: item })
        : budgetPlanStore.saveBudgetPlanItem({ item: item });

    saving.then(saved => {
        submitting.value = false;
        showState.value = false;

        if (resolveFunc) {
            resolveFunc(saved);
        }
    }).catch(error => {
        submitting.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function cancel(): void {
    showState.value = false;

    if (rejectFunc) {
        rejectFunc();
    }
}

defineExpose({
    open
});
</script>

<style scoped>
/* Two halves of one row, standing level with the amount field beside it. Both sizes are
   overridden rather than merely set, because the square the theme wants is an !important one. */
.budget-plan-type-toggle.v-btn-toggle > .v-btn {
    flex: 1 1 0;
    min-width: 0;
    inline-size: auto !important;
    block-size: 56px !important;
}
</style>
