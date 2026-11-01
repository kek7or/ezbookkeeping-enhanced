<template>
    <v-row class="match-height">
        <v-col cols="12">
            <v-card>
                <template #title>
                    <div class="title-and-toolbar d-flex align-center flex-wrap gap-2">
                        <span>{{ tt('Wishlist') }}</span>
                        <v-btn density="compact" color="default" variant="text" size="24"
                               class="ms-2" :icon="true" :loading="loading" @click="load">
                            <template #loader>
                                <v-progress-circular indeterminate size="20"/>
                            </template>
                            <v-icon :icon="mdiRefresh" size="24" />
                            <v-tooltip activator="parent">{{ tt('Refresh') }}</v-tooltip>
                        </v-btn>
                        <v-spacer/>
                        <v-btn color="default" variant="text" :prepend-icon="mdiCalculatorVariantOutline"
                               to="/budget/plan">{{ tt('Budget Plan') }}</v-btn>
                        <v-btn color="primary" variant="text" :prepend-icon="mdiCalendarArrowLeft"
                               :disabled="loading" @click="grabFromBudget">{{ tt('Grab from Budget') }}</v-btn>
                        <v-btn color="primary" variant="tonal" :prepend-icon="mdiFolderPlusOutline"
                               :disabled="loading" @click="addGroup">{{ tt('Add Group') }}</v-btn>
                        <v-btn color="primary" :prepend-icon="mdiPlus"
                               :disabled="loading" @click="addWish('')">{{ tt('Add to Wishlist') }}</v-btn>
                    </div>
                </template>
                <v-card-text class="pt-0">
                    <span class="text-body-2 text-medium-emphasis">{{ tt('Everything you want, sorted into groups. Drag to reorder or move between groups. Attach a wish to a month to try it there, or plan it to count it.') }}</span>
                    <div class="text-body-1 mt-2" v-if="!loading">
                        {{ tt('Total') }}: <span class="font-weight-medium">{{ displayAmount(totalOf(openWishes)) }}</span>
                    </div>
                </v-card-text>
            </v-card>
        </v-col>

        <v-col cols="12" v-if="loading">
            <v-card><v-card-text><v-skeleton-loader type="paragraph" :loading="true"/></v-card-text></v-card>
        </v-col>

        <!-- A board: one column per group, wishes stacked as cards. Ungrouped is always the first
             column and is not itself a group, so it cannot be dragged. -->
        <v-col cols="12" v-if="!loading">
            <div class="wishlist-board">
                <wish-column :title="tt('Ungrouped')" :total="displayAmount(totalOf(ungroupedList.wishes))"
                             :planned-total="displayAmount(totalOf(plannedOf('')))"
                             :list="ungroupedList" :planned="plannedOf('')" :display-amount="displayWishAmount" :display-month="displayWishMonth" :category-name="categoryName"
                             @update="ungroupedList.wishes = $event"
                             @moved="onWishListChange" @attach="pickMonth($event, 'attach')" @plan="pickMonth($event, 'plan')"
                             @detach="detach" @unplan="unplan" @buy="buy" @edit="editWish" @remove="removeWish">
                    <template #actions>
                        <v-btn density="comfortable" size="small" variant="text" :icon="true" @click="addWish('')">
                            <v-icon :icon="mdiPlus"/>
                            <v-tooltip activator="parent">{{ tt('Add to Wishlist') }}</v-tooltip>
                        </v-btn>
                    </template>
                </wish-column>

                <draggable-list class="wishlist-board-groups" item-key="id" handle=".group-drag-handle" ghost-class="dragging-item"
                                v-model="wishlistStore.groups" @change="onGroupsChange">
                    <template #item="{ element: group }">
                        <wish-column :title="group.name" :total="displayAmount(totalOf(listOf(group.id).wishes))"
                                     :planned-total="displayAmount(totalOf(plannedOf(group.id)))"
                                     :list="listOf(group.id)" :planned="plannedOf(group.id)" :display-amount="displayWishAmount" :display-month="displayWishMonth" :category-name="categoryName"
                                     @update="listOf(group.id).wishes = $event"
                                     @moved="onWishListChange" @attach="pickMonth($event, 'attach')" @plan="pickMonth($event, 'plan')"
                                     @detach="detach" @unplan="unplan" @buy="buy" @edit="editWish" @remove="removeWish">
                            <template #handle>
                                <v-icon class="group-drag-handle me-1" size="20" :icon="mdiDrag"/>
                            </template>
                            <template #actions>
                                <v-btn density="comfortable" size="small" variant="text" :icon="true" @click="addWish(group.id)">
                                    <v-icon :icon="mdiPlus"/>
                                    <v-tooltip activator="parent">{{ tt('Add to Wishlist') }}</v-tooltip>
                                </v-btn>
                                <v-btn density="comfortable" size="small" variant="text" :icon="true">
                                    <v-icon :icon="mdiDotsVertical"/>
                                    <v-menu activator="parent">
                                        <v-list>
                                            <v-list-item :prepend-icon="mdiContentCopy" :title="tt('Copy as Text')" :disabled="!listOf(group.id).wishes.length" @click="copyGroup(group)"/>
                                            <v-list-item :prepend-icon="mdiPencilOutline" :title="tt('Rename')" @click="renameGroup(group)"/>
                                            <v-list-item :prepend-icon="mdiDeleteOutline" :title="tt('Delete')" @click="removeGroup(group)"/>
                                        </v-list>
                                    </v-menu>
                                </v-btn>
                            </template>
                        </wish-column>
                    </template>
                </draggable-list>
            </div>
        </v-col>
    </v-row>

    <edit-wish-dialog ref="editWishDialog"/>
    <edit-dialog ref="editTransactionDialog" :type="TransactionEditPageType.Transaction"/>
    <grab-from-budget-dialog ref="grabFromBudgetDialog"/>
    <month-selection-dialog :title="monthAction === 'plan' ? tt('Plan for Month') : tt('Attach to Month')"
                            :hint="monthAction === 'plan' ? tt('The wish counts in that month\'s budget and stays on the wishlist, shown as planned.') : tt('The wish shows on that month\'s budget page to be tried against it, and counts towards nothing.')"
                            :model-value="monthPickerValue"
                            v-model:show="showMonthPicker"
                            @update:model-value="onMonthPicked"
                            @error="onMonthPickerError"/>
    <rename-dialog ref="renameDialog" :label="tt('Group Name')" :default-title="tt('Rename Group')"/>
    <confirm-dialog ref="confirmDialog"/>
    <snack-bar ref="snackbar"/>
</template>

<script setup lang="ts">
import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import RenameDialog from '@/components/desktop/RenameDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';
import EditDialog from '@/views/desktop/transactions/list/dialogs/EditDialog.vue';
import EditWishDialog from './dialogs/EditWishDialog.vue';
import GrabFromBudgetDialog from './dialogs/GrabFromBudgetDialog.vue';
import WishColumn, { type WishList } from './WishColumn.vue';

import { ref, computed, watch, nextTick, useTemplateRef, onMounted } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useUserStore } from '@/stores/user.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { useExchangeRatesStore } from '@/stores/exchangeRates.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { useWishlistStore } from '@/stores/wishlist.ts';

import type { BigDecimal } from '@/core/numeral.ts';
import { TransactionType } from '@/core/transaction.ts';
import { TransactionEditPageType } from '@/views/base/transactions/TransactionEditPageBase.ts';
import type { Year0BasedMonth } from '@/core/datetime.ts';
import { type BudgetWishGroup, BudgetPlanItem } from '@/models/budget_plan.ts';

import { BIG_DECIMAL_ZERO, parseBigDecimal } from '@/lib/numeral.ts';
import { getCurrentDateTime, parseDateTimeFromUnixTime, getYearMonthFirstUnixTime } from '@/lib/datetime.ts';
import { copyTextToClipboard } from '@/lib/ui/common.ts';

import {
    mdiRefresh,
    mdiPlus,
    mdiDrag,
    mdiDotsVertical,
    mdiPencilOutline,
    mdiDeleteOutline,
    mdiContentCopy,
    mdiCalendarArrowLeft,
    mdiFolderPlusOutline,
    mdiCalculatorVariantOutline
} from '@mdi/js';

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type RenameDialogType = InstanceType<typeof RenameDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;
type EditWishDialogType = InstanceType<typeof EditWishDialog>;
type EditDialogType = InstanceType<typeof EditDialog>;
type GrabFromBudgetDialogType = InstanceType<typeof GrabFromBudgetDialog>;

const { tt, formatAmountToLocalizedNumeralsWithCurrency, formatDateTimeToGregorianLikeLongYearMonth } = useI18n();

const userStore = useUserStore();
const accountsStore = useAccountsStore();
const exchangeRatesStore = useExchangeRatesStore();
const transactionCategoriesStore = useTransactionCategoriesStore();
const wishlistStore = useWishlistStore();

const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const renameDialog = useTemplateRef<RenameDialogType>('renameDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');
const editWishDialog = useTemplateRef<EditWishDialogType>('editWishDialog');
const editTransactionDialog = useTemplateRef<EditDialogType>('editTransactionDialog');
const grabFromBudgetDialog = useTemplateRef<GrabFromBudgetDialogType>('grabFromBudgetDialog');

const loading = ref<boolean>(true);
const ungroupedList = ref<WishList>({ groupId: '', wishes: [] });
const groupLists = ref<Record<string, WishList>>({});
// what was planned from the wishlist, by group, shown under each group and kept out of dragging
const plannedLists = ref<Record<string, BudgetPlanItem[]>>({});

const showMonthPicker = ref<boolean>(false);
const monthAction = ref<'attach' | 'plan'>('attach');
const monthPickerWish = ref<BudgetPlanItem | null>(null);
const monthPickerValue = ref<Year0BasedMonth | undefined>(undefined);

const defaultCurrency = computed<string>(() => userStore.currentUserDefaultCurrency);
// the total is what is still only wanted; what is planned already counts in its month
const openWishes = computed<BudgetPlanItem[]>(() => wishlistStore.wishes.filter(wish => wish.wished));

// The draggable lists need arrays of their own to move items between, so they are rebuilt from the
// store whenever it changes rather than computed from it. A wish whose group is gone is ungrouped.
function rebuildLists(): void {
    const lists: Record<string, WishList> = {};
    const ungrouped: WishList = { groupId: '', wishes: [] };
    const ungroupedPlanned: BudgetPlanItem[] = [];
    const planned: Record<string, BudgetPlanItem[]> = { '': ungroupedPlanned };

    for (const group of wishlistStore.groups) {
        lists[group.id] = { groupId: group.id, wishes: [] };
        planned[group.id] = [];
    }

    const sorted = [...wishlistStore.wishes].sort((a, b) => a.displayOrder - b.displayOrder);

    for (const wish of sorted) {
        if (wish.wished) {
            (lists[wish.wishGroupId] ?? ungrouped).wishes.push(wish);
        } else {
            (planned[wish.wishGroupId] ?? ungroupedPlanned).push(wish);
        }
    }

    for (const list of Object.values(planned)) {
        list.sort((a, b) => a.year - b.year || a.month - b.month);
    }

    groupLists.value = lists;
    ungroupedList.value = ungrouped;
    plannedLists.value = planned;
}

watch(() => [wishlistStore.groups.length, wishlistStore.wishes], rebuildLists, { deep: true });

function listOf(groupId: string): WishList {
    return groupLists.value[groupId] ?? { groupId: groupId, wishes: [] };
}

function plannedOf(groupId: string): BudgetPlanItem[] {
    return plannedLists.value[groupId] ?? [];
}

// a subcategory is shown under its parent, since "Furniture" alone may sit under more than one
function categoryName(categoryId: string): string {
    const category = transactionCategoriesStore.allTransactionCategoriesMap[categoryId];

    if (!category) {
        return tt('Unknown Category');
    }

    const parent = category.parentId && category.parentId !== '0' ? transactionCategoriesStore.allTransactionCategoriesMap[category.parentId] : undefined;

    return parent ? `${parent.name} / ${category.name}` : category.name;
}

function wishCurrency(wish: BudgetPlanItem): string {
    return accountsStore.allAccountsMap[wish.accountId]?.currency ?? defaultCurrency.value;
}

// Group totals are in the user's own currency. A wish priced in a currency with no rate is left out
// rather than counted at the wrong value.
function totalOf(wishes: BudgetPlanItem[]): BigDecimal {
    let total = BIG_DECIMAL_ZERO;

    for (const wish of wishes) {
        const currency = wishCurrency(wish);
        const amount = parseBigDecimal(wish.amount);
        const converted = currency === defaultCurrency.value ? amount : exchangeRatesStore.getExchangedAmount(amount, currency, defaultCurrency.value);

        if (converted) {
            total = total.add(converted);
        }
    }

    return total;
}

function displayAmount(amount: BigDecimal): string {
    return formatAmountToLocalizedNumeralsWithCurrency(amount.truncate(), defaultCurrency.value);
}

function displayWishAmount(wish: BudgetPlanItem): string {
    return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(wish.amount).truncate(), wishCurrency(wish));
}

function displayWishMonth(wish: BudgetPlanItem): string {
    return formatDateTimeToGregorianLikeLongYearMonth(parseDateTimeFromUnixTime(getYearMonthFirstUnixTime({ year: wish.year, month0base: wish.month - 1 })));
}

// A group as plain text to paste into a message: the heading with its total, then each wish with its
// price and link. Descriptions are left out, since they are often notes meant for nobody else.
function copyGroup(group: BudgetWishGroup): void {
    const wishes = listOf(group.id).wishes;
    const lines = [`${group.name} — ${displayAmount(totalOf(wishes))}`, ''];

    for (const wish of wishes) {
        lines.push(`• ${wish.name} — ${displayWishAmount(wish)}`);

        if (wish.url) {
            lines.push(`  ${wish.url}`);
        }
    }

    copyTextToClipboard(lines.join('\n'));
    snackbar.value?.showMessage('Copied');
}

// the stores reject with what the snack bar shows, marked processed when it has been shown already
function showError(error: { message: string } & { processed?: boolean }): void {
    if (!error.processed) {
        snackbar.value?.showError(error);
    }
}

function load(): void {
    loading.value = true;

    Promise.all([
        accountsStore.loadAllAccounts({ force: false }),
        transactionCategoriesStore.loadAllCategories({ force: false }),
        wishlistStore.loadWishlist()
    ]).then(() => {
        rebuildLists();
        loading.value = false;
    }).catch(error => {
        loading.value = false;
        showError(error);
    });
}

// A drag between two lists changes both, and both report it. They are saved together once the drag
// has settled, so the move is one request.
const changedLists = new Set<WishList>();

function onWishListChange(list: WishList): void {
    const wasEmpty = changedLists.size === 0;
    changedLists.add(list);

    if (!wasEmpty) {
        return;
    }

    nextTick(() => {
        const lists = [...changedLists];
        changedLists.clear();
        wishlistStore.moveWishes(lists).catch(error => {
            showError(error);
            load();
        });
    });
}

function onGroupsChange(): void {
    wishlistStore.moveGroups().catch(error => {
        showError(error);
        load();
    });
}

function addWish(groupId: string): void {
    editWishDialog.value?.open(BudgetPlanItem.createNewWish(0, 0, groupId)).then(() => {
        snackbar.value?.showMessage('You have added this to the wishlist');
    }).catch(() => {
        // dismissed
    });
}

function editWish(wish: BudgetPlanItem): void {
    editWishDialog.value?.open(wish).then(() => {
        snackbar.value?.showMessage('You have saved this wish');
    }).catch(() => {
        // dismissed
    });
}

function removeWish(wish: BudgetPlanItem): void {
    confirmDialog.value?.open('Are you sure you want to remove this from the wishlist?').then(() => {
        wishlistStore.deleteWish({ wish: wish }).then(() => {
            snackbar.value?.showMessage('You have removed this from the wishlist');
        }).catch(showError);
    }).catch(() => {
        // dismissed
    });
}

function detach(wish: BudgetPlanItem): void {
    wishlistStore.attachWish({ wish: wish, year: 0, month: 0 }).catch(showError);
}

function unplan(wish: BudgetPlanItem): void {
    wishlistStore.grabPlannedItem({ item: wish }).then(() => {
        snackbar.value?.showMessage('You have taken this back out of the month');
    }).catch(showError);
}

// Opens the transaction editor filled in from the wish, and takes the wish off the wishlist only once
// the transaction is saved. It stays planned in its month, to measure the purchase against.
function buy(wish: BudgetPlanItem): void {
    editTransactionDialog.value?.open({
        type: TransactionType.Expense,
        categoryId: wish.categoryId || undefined,
        accountId: wish.accountId || undefined,
        amount: wish.amount,
        comment: wish.name,
        noTransactionDraft: true
    }).then(result => {
        if (!result || !result.transactionId) {
            return;
        }

        wishlistStore.buyWish({ item: wish, transactionId: result.transactionId }).then(() => {
            snackbar.value?.showMessage('You have added the transaction and taken this off the wishlist');
        }).catch(showError);
    }).catch(() => {
        // dismissed
    });
}

function pickMonth(wish: BudgetPlanItem, action: 'attach' | 'plan'): void {
    const now = getCurrentDateTime();

    monthAction.value = action;
    monthPickerWish.value = wish;
    monthPickerValue.value = wish.year
        ? { year: wish.year, month0base: wish.month - 1 }
        : { year: now.getGregorianCalendarYear(), month0base: now.getGregorianCalendarMonth() - 1 };
    showMonthPicker.value = true;
}

function onMonthPicked(value: Year0BasedMonth): void {
    const wish = monthPickerWish.value;
    showMonthPicker.value = false;

    if (!wish) {
        return;
    }

    const year = value.year;
    const month = value.month0base + 1;

    if (monthAction.value === 'plan') {
        wishlistStore.planWish({ wish: wish, year: year, month: month }).then(() => {
            snackbar.value?.showMessage('You have put this into the month');
        }).catch(showError);
    } else {
        wishlistStore.attachWish({ wish: wish, year: year, month: month }).catch(showError);
    }
}

function onMonthPickerError(message: string): void {
    snackbar.value?.showError(message);
}

function grabFromBudget(): void {
    grabFromBudgetDialog.value?.open();
}

function addGroup(): void {
    renameDialog.value?.open('', tt('Add Group')).then(name => {
        wishlistStore.addGroup({ name: name }).catch(showError);
    }).catch(() => {
        // dismissed
    });
}

function renameGroup(group: BudgetWishGroup): void {
    renameDialog.value?.open(group.name, tt('Rename Group')).then(name => {
        wishlistStore.renameGroup({ group: group, name: name }).catch(showError);
    }).catch(() => {
        // dismissed
    });
}

function removeGroup(group: BudgetWishGroup): void {
    confirmDialog.value?.open('Are you sure you want to delete this group? Its wishes become ungrouped.').then(() => {
        wishlistStore.deleteGroup({ group: group }).catch(showError);
    }).catch(() => {
        // dismissed
    });
}

onMounted(() => {
    load();
});
</script>

<style scoped>
/* columns side by side, scrolling sideways once there are more than fit */
.wishlist-board {
    display: flex;
    align-items: flex-start;
    gap: 16px;
    overflow-x: auto;
    padding-bottom: 8px;
}

.wishlist-board-groups {
    display: flex;
    align-items: flex-start;
    gap: 16px;
}

.group-drag-handle {
    cursor: grab;
}
</style>
