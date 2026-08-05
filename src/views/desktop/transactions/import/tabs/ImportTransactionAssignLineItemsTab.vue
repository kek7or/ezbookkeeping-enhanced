<template>
    <div class="import-receipt-line-items">
        <div class="d-flex align-center text-body-2 text-medium-emphasis mb-4">
            <v-icon class="me-1" size="18" :icon="mdiDrag"/>
            <span>{{ tt('Drag an item to another category to move what it cost') }}</span>
        </div>

        <div class="import-receipt mb-6" :key="receipt.index" v-for="receipt in receipts">
            <div class="d-flex align-center flex-wrap gap-2 mb-3">
                <v-chip size="small" label variant="tonal" :prepend-icon="mdiReceiptTextOutline">{{ receipt.fileName }}</v-chip>
                <span class="text-body-2">{{ getReceiptDisplayDateTime(receipt) }}</span>
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
                <v-spacer/>
                <span class="text-body-2 text-medium-emphasis">
                    {{ tt('format.misc.receiptLineItemCount', { count: formatNumberToLocalizedNumerals(receipt.lineItemCount) }) }}
                </span>
                <span class="text-h6">{{ getDisplayAmount(receipt.totalAmount, getReceiptCurrency(receipt)) }}</span>
            </div>

            <div class="d-flex align-start flex-wrap gap-4">
                <div class="import-receipt-category" :key="categoryGroup.id" v-for="categoryGroup in receipt.categoryGroups">
                    <div class="import-receipt-category-header pa-3">
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
                        <div class="d-flex align-center mt-1">
                            <span class="text-h6">{{ getDisplayAmount(categoryGroup.totalAmount, getReceiptCurrency(receipt)) }}</span>
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
                                    item-key="id"
                                    handle=".import-receipt-line-item-handle"
                                    ghost-class="import-receipt-line-item-ghost"
                                    :group="`receiptLineItems_${receipt.index}`"
                                    :animation="150"
                                    :disabled="!!disabled"
                                    v-model="categoryGroup.lineItems">
                        <template #item="{ element }">
                            <div class="import-receipt-line-item d-flex align-center px-2 py-1 mb-2">
                                <div class="import-receipt-line-item-handle d-flex align-center flex-grow-1 overflow-hidden">
                                    <v-icon class="me-1 flex-grow-0" size="18" :icon="mdiDrag"/>
                                    <span class="text-truncate">{{ element.name }}</span>
                                    <v-tooltip activator="parent" open-delay="500">{{ element.name }}</v-tooltip>
                                </div>
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

                <div class="import-receipt-category import-receipt-category-new">
                    <div class="pa-3">
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
                                           :model-value="''"
                                           @update:model-value="addCategoryGroup(receipt, $event as string)">
                        </two-column-select>
                        <div class="text-body-2 text-medium-emphasis mt-2">{{ tt('Add a category to move items into') }}</div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue';

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
import { parseDateTimeFromUnixTimeWithTimezoneOffset } from '@/lib/datetime.ts';
import { getTransactionPrimaryCategoryName, getTransactionSecondaryCategoryName } from '@/lib/category.ts';

import {
    mdiDrag,
    mdiPound,
    mdiClose,
    mdiAlertOutline,
    mdiReceiptTextOutline
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
    formatDateTimeToLongDateTime,
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

function getReceiptDisplayDateTime(receipt: ImportReceipt): string {
    return formatDateTimeToLongDateTime(parseDateTimeFromUnixTimeWithTimezoneOffset(receipt.time, receipt.utcOffset));
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

    .import-receipt-category {
        width: 320px;
        border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
        border-radius: 6px;
    }

    .import-receipt-category-header {
        border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
    }

    /* the column is a drop target for the whole of its height, so a line can be dropped onto a
       category without having to aim at the lines already in it */
    .import-receipt-category-items {
        min-height: 96px;
    }

    .import-receipt-category-empty {
        height: 72px;
        border: 1px dashed rgba(var(--v-border-color), var(--v-border-opacity));
        border-radius: 4px;
    }

    .import-receipt-category-new {
        border-style: dashed;
    }

    .import-receipt-line-item {
        border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
        border-radius: 4px;
        background-color: rgba(var(--v-theme-surface));
    }

    .import-receipt-line-item:hover {
        border-color: rgba(var(--v-theme-primary), 0.5);
    }

    .import-receipt-line-item-handle {
        cursor: grab;
        min-height: 32px;
    }

    .import-receipt-line-item-ghost {
        opacity: 0.5;
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
</style>
