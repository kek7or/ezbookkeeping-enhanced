<template>
    <div class="import-receipt-line-items" :class="{ 'import-receipt-line-items-compact': compactView }">
        <div class="d-flex align-center text-body-2 text-medium-emphasis mb-4">
            <v-icon class="me-1" size="18" :icon="mdiDrag"/>
            <span>{{ tt('Drag an item to another category to move what it cost') }}</span>
        </div>

        <div class="import-receipt mb-6" :key="receipt.index" v-for="receipt in receipts">
            <div class="import-receipt-header d-flex align-center flex-wrap gap-2 mb-3 py-2">
                <v-chip size="small" label variant="tonal" :prepend-icon="mdiReceiptTextOutline">{{ receipt.fileName }}</v-chip>
                <div class="import-receipt-time">
                    <date-time-select density="compact" variant="plain"
                                      :disabled="!!disabled"
                                      :timezone-utc-offset="receipt.utcOffset"
                                      :model-value="receipt.time"
                                      @update:model-value="receipt.time = $event">
                    </date-time-select>
                    <v-tooltip activator="parent" open-delay="500">{{ tt('Every transaction of this receipt is booked at this time') }}</v-tooltip>
                </div>
                <div style="width: 220px">
                    <two-column-select density="compact" variant="plain"
                                       primary-key-field="id" primary-value-field="category"
                                       primary-title-field="name" primary-footer-field="displayBalance"
                                       primary-icon-field="icon" primary-icon-type="account"
                                       primary-sub-items-field="accounts"
                                       :primary-title-i18n="true"
                                       secondary-key-field="id" secondary-value-field="id"
                                       secondary-title-field="name" secondary-footer-field="displayBalance"
                                       secondary-icon-field="icon" secondary-icon-type="account" secondary-color-field="color"
                                       :disabled="!!disabled || !allVisibleAccounts.length"
                                       :enable-filter="true" :filter-placeholder="tt('Find account')" :filter-no-items-text="tt('No available account')"
                                       :custom-selection-primary-text="getReceiptAccountDisplayName(receipt)"
                                       :placeholder="tt('Account')"
                                       :items="allVisibleCategorizedAccounts"
                                       v-model="receipt.sourceAccountId">
                    </two-column-select>
                </div>
                <div class="import-receipt-add-category d-flex align-center px-2">
                    <v-icon class="me-1 text-medium-emphasis" size="18" :icon="mdiPlus"/>
                    <two-column-select density="compact" variant="plain"
                                       primary-key-field="id" primary-value-field="id" primary-title-field="name"
                                       primary-icon-field="icon" primary-icon-type="category" primary-color-field="color"
                                       primary-hidden-field="hidden" primary-sub-items-field="subCategories"
                                       secondary-key-field="id" secondary-value-field="id" secondary-title-field="name"
                                       secondary-icon-field="icon" secondary-icon-type="category" secondary-color-field="color"
                                       secondary-hidden-field="hidden"
                                       :disabled="!!disabled || !hasVisibleExpenseCategories"
                                       :enable-filter="true" :filter-placeholder="tt('Find category')" :filter-no-items-text="tt('No available category')"
                                       :custom-selection-primary-text="tt('Add Category')"
                                       :placeholder="tt('Add Category')"
                                       :items="allCategories[CategoryType.Expense]"
                                       :expand-primary-only="true"
                                       :model-value="''"
                                       @update:model-value="addCategoryGroup(receipt, $event as string)">
                    </two-column-select>
                    <v-tooltip activator="parent" open-delay="500">{{ tt('Add a category to move items into') }}</v-tooltip>
                </div>
                <v-spacer/>
                <span class="text-body-2 text-medium-emphasis">
                    {{ tt('format.misc.receiptLineItemCount', { count: formatNumberToLocalizedNumerals(receipt.lineItemCount) }) }}
                </span>
                <span class="text-h6">{{ getDisplayAmount(receipt.totalAmount, getReceiptCurrency(receipt)) }}</span>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1"
                       :icon="true" @click="compactView = !compactView">
                    <v-icon size="20" :icon="compactView ? mdiUnfoldMoreHorizontal : mdiUnfoldLessHorizontal"/>
                    <v-tooltip activator="parent">{{ compactView ? tt('Show Taller Categories') : tt('Show Shorter Categories') }}</v-tooltip>
                </v-btn>
            </div>

            <div class="import-receipt-categories">
                <div class="import-receipt-category" :key="categoryGroup.id" v-for="categoryGroup in receipt.categoryGroups">
                    <div class="import-receipt-category-header px-2 py-1">
                        <div class="d-flex align-center">
                            <v-checkbox-btn density="compact" class="flex-grow-0 me-1"
                                            :color="isCategoryGroupValid(receipt, categoryGroup) ? 'primary' : 'error'"
                                            :disabled="!!disabled || !categoryGroup.isImportable"
                                            v-model="categoryGroup.selected">
                                <v-tooltip activator="parent">{{ tt('Import This Transaction') }}</v-tooltip>
                            </v-checkbox-btn>
                            <ItemIcon size="24px" icon-type="category"
                                      :icon-id="allCategoriesMap[categoryGroup.categoryId]?.icon ?? ''"
                                      :color="allCategoriesMap[categoryGroup.categoryId]?.color ?? ''"
                                      v-if="allCategoriesMap[categoryGroup.categoryId]"></ItemIcon>
                            <v-icon class="text-error" :icon="mdiAlertOutline" v-else></v-icon>
                            <div class="flex-grow-1 ms-1 overflow-hidden">
                                <two-column-select density="compact" variant="plain"
                                                   primary-key-field="id" primary-value-field="id" primary-title-field="name"
                                                   primary-icon-field="icon" primary-icon-type="category" primary-color-field="color"
                                                   primary-hidden-field="hidden" primary-sub-items-field="subCategories"
                                                   secondary-key-field="id" secondary-value-field="id" secondary-title-field="name"
                                                   secondary-icon-field="icon" secondary-icon-type="category" secondary-color-field="color"
                                                   secondary-hidden-field="hidden"
                                                   :disabled="!!disabled || !hasVisibleExpenseCategories"
                                                   :enable-filter="true" :filter-placeholder="tt('Find category')" :filter-no-items-text="tt('No available category')"
                                                   :show-selection-primary-text="true"
                                                   :custom-selection-primary-text="getPrimaryCategoryName(categoryGroup)"
                                                   :custom-selection-secondary-text="getSecondaryCategoryName(categoryGroup)"
                                                   :placeholder="categoryGroup.originalCategoryName || tt('Category')"
                                                   :items="allCategories[CategoryType.Expense]"
                                                   v-model="categoryGroup.categoryId">
                                </two-column-select>
                            </div>
                            <v-btn density="compact" color="default" variant="text" size="24" class="ms-1"
                                   :icon="true" :disabled="!!disabled"
                                   v-if="!categoryGroup.lineItems.length"
                                   @click="removeCategoryGroup(receipt, categoryGroup)">
                                <v-icon :icon="mdiClose" size="18"/>
                                <v-tooltip activator="parent">{{ tt('Remove') }}</v-tooltip>
                            </v-btn>
                        </div>
                        <div class="d-flex align-center">
                            <span class="text-subtitle-1 font-weight-medium">{{ getDisplayAmount(categoryGroup.totalAmount, getReceiptCurrency(receipt)) }}</span>
                            <!-- a refund carries the category its deposit was charged to, so it would
                                 otherwise be told apart from that category's purchases by its sign alone -->
                            <v-chip size="x-small" label variant="tonal" class="ms-2"
                                    :prepend-icon="mdiCashRefund"
                                    v-if="categoryGroup.refund">{{ tt('Refund') }}</v-chip>
                            <span class="text-caption text-medium-emphasis ms-2 text-no-wrap" v-else-if="categoryGroup.lineItems.length">
                                {{ tt('format.misc.receiptLineItemCount', { count: formatNumberToLocalizedNumerals(categoryGroup.lineItems.length) }) }}
                            </span>
                            <v-spacer/>
                            <div class="import-receipt-category-tags">
                                <v-autocomplete item-title="name" item-value="id"
                                                auto-select-first persistent-placeholder multiple chips closable-chips
                                                density="compact" variant="plain"
                                                :disabled="!!disabled"
                                                :placeholder="tt('Tags')"
                                                :items="allTagsWithGroupHeader"
                                                :no-data-text="tt('No available tag')"
                                                v-model="categoryGroup.tagIds">
                                    <template #chip="{ props: chipProps, index }">
                                        <v-chip :prepend-icon="mdiPound"
                                                :text="allTagsMap[categoryGroup.tagIds[index] as string]?.name"
                                                v-bind="chipProps"/>
                                    </template>

                                    <template #subheader="{ props: subheaderProps }">
                                        <v-list-subheader>{{ subheaderProps['title'] }}</v-list-subheader>
                                    </template>

                                    <template #item="{ props: itemProps, item }">
                                        <v-list-item :value="item.value" v-bind="itemProps" v-if="item.raw instanceof TransactionTag && !item.raw.hidden">
                                            <template #title>
                                                <v-list-item-title>
                                                    <div class="d-flex align-center">
                                                        <v-icon size="20" start :icon="mdiPound"/>
                                                        <span>{{ item.title }}</span>
                                                    </div>
                                                </v-list-item-title>
                                            </template>
                                        </v-list-item>
                                    </template>
                                </v-autocomplete>
                            </div>
                        </div>
                    </div>

                    <draggable-list class="import-receipt-category-items pa-2"
                                    :scroll-sensitivity="80"
                                    item-key="id"
                                    handle=".import-receipt-line-item-handle"
                                    ghost-class="import-receipt-line-item-ghost"
                                    :group="`receiptLineItems_${receipt.index}`"
                                    :animation="150"
                                    :disabled="!!disabled"
                                    v-model="categoryGroup.lineItems">
                        <template #item="{ element }">
                            <div class="import-receipt-line-item d-flex align-center px-2 mb-1">
                                <div class="import-receipt-line-item-handle d-flex align-center flex-grow-1 overflow-hidden">
                                    <v-icon class="me-1 flex-grow-0" size="18" :icon="mdiDrag"/>
                                    <span class="text-truncate">{{ element.name }}</span>
                                    <v-tooltip activator="parent" open-delay="500">{{ element.name }}</v-tooltip>
                                </div>
                                <span class="import-receipt-line-item-remembered d-flex align-center ms-1"
                                      v-if="element.remembered">
                                    <v-icon size="16" :icon="mdiHistory"/>
                                    <v-tooltip activator="parent" open-delay="500">{{ tt('Filed here from an earlier receipt, drag it away to change that') }}</v-tooltip>
                                </span>
                                <div class="import-receipt-line-item-amount ms-2">
                                    <amount-input density="compact" variant="plain"
                                                  :currency="getReceiptCurrency(receipt)"
                                                  :show-currency="true"
                                                  :disabled="!!disabled"
                                                  v-model="element.amount"/>
                                </div>
                            </div>
                        </template>
                        <template #footer>
                            <div class="import-receipt-category-empty d-flex align-center justify-center text-body-2 text-medium-emphasis"
                                 v-if="!categoryGroup.lineItems.length">
                                {{ tt('Drop items here') }}
                            </div>
                        </template>
                    </draggable-list>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { useTransactionTagSelectionBase } from '@/components/base/TransactionTagSelectionBase.ts';

import { useSettingsStore } from '@/stores/setting.ts';
import { useUserStore } from '@/stores/user.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { useTransactionTagsStore } from '@/stores/transactionTag.ts';

import { CategoryType } from '@/core/category.ts';

import { Account, type CategorizedAccountWithDisplayBalance } from '@/models/account.ts';
import type { TransactionCategory } from '@/models/transaction_category.ts';
import { TransactionTag } from '@/models/transaction_tag.ts';
import { ImportTransaction } from '@/models/imported_transaction.ts';
import { ImportReceipt, ImportReceiptCategoryGroup } from '@/models/imported_receipt.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { getTransactionPrimaryCategoryName, getTransactionSecondaryCategoryName } from '@/lib/category.ts';

import {
    mdiDrag,
    mdiPlus,
    mdiPound,
    mdiCashRefund,
    mdiClose,
    mdiAlertOutline,
    mdiReceiptTextOutline,
    mdiHistory,
    mdiUnfoldLessHorizontal,
    mdiUnfoldMoreHorizontal
} from '@mdi/js';

const props = defineProps<{
    receipts: ImportReceipt[];
    disabled?: boolean;
}>();

const emit = defineEmits<{
    (e: 'update:transactions', value: ImportTransaction[]): void;
}>();

const {
    tt,
    formatAmountToLocalizedNumeralsWithCurrency,
    formatNumberToLocalizedNumerals,
    getCategorizedAccountsWithDisplayBalance
} = useI18n();

const { allTagsWithGroupHeader } = useTransactionTagSelectionBase({ modelValue: [] }, false);

const settingsStore = useSettingsStore();
const userStore = useUserStore();
const accountsStore = useAccountsStore();
const transactionCategoriesStore = useTransactionCategoriesStore();
const transactionTagsStore = useTransactionTagsStore();

// a long receipt fills the screen with columns, so its categories can be shrunk to a fixed height that
// scrolls on its own - the whole board then fits on one screen and every column stays a drop target
const compactView = ref<boolean>(false);

const showAccountBalance = computed<boolean>(() => settingsStore.appSettings.showAccountBalance);
const customAccountCategoryOrder = computed<string>(() => settingsStore.appSettings.accountCategoryOrders);

const defaultCurrency = computed<string>(() => userStore.currentUserDefaultCurrency);

const allVisibleAccounts = computed<Account[]>(() => accountsStore.allVisiblePlainAccounts);
const allVisibleCategorizedAccounts = computed<CategorizedAccountWithDisplayBalance[]>(() => getCategorizedAccountsWithDisplayBalance(allVisibleAccounts.value, showAccountBalance.value, customAccountCategoryOrder.value));
const allAccountsMap = computed<Record<string, Account>>(() => accountsStore.allAccountsMap);
const allCategories = computed<Record<number, TransactionCategory[]>>(() => transactionCategoriesStore.allTransactionCategories);
const allCategoriesMap = computed<Record<string, TransactionCategory>>(() => transactionCategoriesStore.allTransactionCategoriesMap);
const allTagsMap = computed<Record<string, TransactionTag>>(() => transactionTagsStore.allTransactionTagsMap);
const hasVisibleExpenseCategories = computed<boolean>(() => transactionCategoriesStore.hasVisibleExpenseCategories);

// the transactions are rebuilt from the receipts on every change, so that dragging a line to another
// category is all it takes for both amounts and both descriptions to follow
const importTransactions = computed<ImportTransaction[]>(() => {
    const transactions: ImportTransaction[] = [];

    for (const receipt of props.receipts) {
        transactions.push(...receipt.toImportTransactions(transactions.length));
    }

    return transactions;
});

const selectedImportTransactionCount = computed<number>(() => importTransactions.value.filter(transaction => transaction.selected).length);
const canImport = computed<boolean>(() => selectedImportTransactionCount.value > 0 && importTransactions.value.every(transaction => !transaction.selected || transaction.valid));

function getReceiptCurrency(receipt: ImportReceipt): string {
    let currency = receipt.originalSourceAccountCurrency || defaultCurrency.value;

    if (receipt.sourceAccountId && receipt.sourceAccountId !== '0' && allAccountsMap.value[receipt.sourceAccountId]) {
        currency = allAccountsMap.value[receipt.sourceAccountId]!.currency;
    }

    return currency;
}

function getDisplayAmount(amount: number, currency: string): string {
    return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(amount), currency);
}

function getReceiptAccountDisplayName(receipt: ImportReceipt): string {
    const account = allAccountsMap.value[receipt.sourceAccountId];
    return account ? account.name : receipt.originalSourceAccountName;
}

function getPrimaryCategoryName(categoryGroup: ImportReceiptCategoryGroup): string {
    return getTransactionPrimaryCategoryName(categoryGroup.categoryId, allCategories.value[CategoryType.Expense] ?? []);
}

function getSecondaryCategoryName(categoryGroup: ImportReceiptCategoryGroup): string {
    return getTransactionSecondaryCategoryName(categoryGroup.categoryId, allCategories.value[CategoryType.Expense] ?? []);
}

// a group only becomes a transaction once it holds something, so an emptied column is not a problem
// to point out - it simply produces nothing
function isCategoryGroupValid(receipt: ImportReceipt, categoryGroup: ImportReceiptCategoryGroup): boolean {
    if (!categoryGroup.isImportable) {
        return true;
    }

    return !!allCategoriesMap.value[categoryGroup.categoryId] && !!allAccountsMap.value[receipt.sourceAccountId];
}

function addCategoryGroup(receipt: ImportReceipt, categoryId: string): void {
    const category = allCategoriesMap.value[categoryId];

    if (!category) {
        return;
    }

    const existedCategoryGroup = receipt.categoryGroups.find(categoryGroup => categoryGroup.categoryId === categoryId);

    if (existedCategoryGroup) {
        return;
    }

    receipt.addCategoryGroup(categoryId, category.name);
}

function removeCategoryGroup(receipt: ImportReceipt, categoryGroup: ImportReceiptCategoryGroup): void {
    if (categoryGroup.lineItems.length > 0) {
        return;
    }

    receipt.categoryGroups = receipt.categoryGroups.filter(existedCategoryGroup => existedCategoryGroup !== categoryGroup);
}

watch(importTransactions, (transactions) => {
    emit('update:transactions', transactions);
}, { immediate: true });

defineExpose({
    canImport
});
</script>

<style>
.import-receipt-line-items {
    /* the controls sit inside cards rather than a form, so the space Vuetify reserves under each one
       for a validation message would only push the columns apart */
    .v-input__details {
        display: none;
    }

    /* what the receipt is and what it comes to stays in sight while its columns are scrolled through */
    .import-receipt-header {
        position: sticky;
        top: 0;
        z-index: 2;
        background-color: rgb(var(--v-theme-surface));
    }

    .import-receipt-time {
        width: 240px;
    }

    .import-receipt-add-category {
        width: 190px;
        border: 1px dashed rgba(var(--v-border-color), var(--v-border-opacity));
        border-radius: 6px;
    }

    /* the columns are laid out on a grid rather than wrapped, so that a category holding many lines
       cannot leave a hole under the short ones standing next to it */
    .import-receipt-categories {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(300px, 360px));
        gap: 12px;
    }

    .import-receipt-category {
        display: flex;
        flex-direction: column;
        min-width: 0;
        max-height: 400px;
        border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
        border-radius: 6px;
    }

    .import-receipt-category-header {
        flex: 0 0 auto;
        border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
    }

    /* the column is a drop target for the whole of its height, so a line can be dropped onto a
       category without having to aim at the lines already in it. A category longer than the column
       scrolls inside it instead of stretching the page */
    .import-receipt-category-items {
        display: flex;
        flex-direction: column;
        flex: 1 1 auto;
        min-height: 84px;
        overflow-y: auto;
    }

    .import-receipt-category-empty {
        flex: 1 1 auto;
        min-height: 64px;
        border: 1px dashed rgba(var(--v-border-color), var(--v-border-opacity));
        border-radius: 4px;
    }

    .import-receipt-line-item {
        flex: 0 0 auto;
        border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
        border-radius: 4px;
        background-color: rgba(var(--v-theme-surface));
    }

    /* a line is only a name and an amount, so it is given the height of the text rather than the
       height an input field would take on its own */
    .import-receipt-line-item .v-field__input,
    .import-receipt-line-item .v-field__prepend-inner {
        min-height: 28px;
        padding-top: 0;
        padding-bottom: 0;
        font-size: 0.875rem;
    }

    .import-receipt-line-item:hover {
        border-color: rgba(var(--v-theme-primary), 0.5);
    }

    .import-receipt-line-item-handle {
        cursor: grab;
        min-height: 30px;
        font-size: 0.875rem;
    }

    .import-receipt-line-item-ghost {
        opacity: 0.5;
    }

    /* a line filed from an earlier receipt is marked, not highlighted: the user still has to be able
       to run their eye down a column of names, and most lines carry this mark once a few shops have
       been imported */
    .import-receipt-line-item-remembered {
        flex: 0 0 auto;
        color: rgba(var(--v-theme-on-surface), var(--v-disabled-opacity));
    }

    .import-receipt-category-tags {
        max-width: 160px;
    }

    .import-receipt-line-item-amount {
        width: 110px;
    }

    .import-receipt-line-item-amount input {
        text-align: end;
    }
}

/* the shorter columns show fewer lines at a time, so that a receipt of any length can be looked over
   as a whole before any of it is corrected */
.import-receipt-line-items-compact .import-receipt-category {
    max-height: 216px;
}
</style>
