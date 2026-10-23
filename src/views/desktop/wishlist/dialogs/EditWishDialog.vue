<template>
    <v-dialog width="600" v-model="showState">
        <v-card class="pa-sm-1 pa-md-2">
            <template #title>
                <h4 class="text-h4 text-wrap">{{ isNew ? tt('Add to Wishlist') : tt('Edit Wish') }}</h4>
            </template>
            <v-card-text>
                <v-row>
                    <v-col cols="12">
                        <v-text-field type="text" persistent-placeholder
                                      autofocus
                                      :disabled="submitting"
                                      :label="tt('Name')"
                                      v-model="name"/>
                    </v-col>
                    <v-col cols="12" :md="isNew ? 6 : 12">
                        <amount-input :currency="currency"
                                      :show-currency="true"
                                      :persistent-placeholder="true"
                                      :disabled="submitting"
                                      :label="tt('Price')"
                                      :enable-formula="true"
                                      v-model="amount"/>
                    </v-col>
                    <v-col cols="12" md="6" v-if="isNew">
                        <v-select item-title="name" item-value="id"
                                  persistent-placeholder
                                  :disabled="submitting"
                                  :label="tt('Group')"
                                  :items="groupOptions"
                                  v-model="groupId"/>
                    </v-col>
                    <v-col cols="12">
                        <v-textarea rows="2" auto-grow persistent-placeholder
                                    :disabled="submitting"
                                    :label="tt('Description')"
                                    v-model="comment"/>
                    </v-col>
                    <v-col cols="12">
                        <v-text-field type="url" persistent-placeholder
                                      :disabled="submitting"
                                      :label="tt('Web Link')"
                                      placeholder="https://"
                                      v-model="url"/>
                    </v-col>
                    <v-col cols="12" class="text-caption text-medium-emphasis pt-0" v-if="original.createdTime">
                        {{ tt('Created') }}: {{ displayTime(original.createdTime) }} · {{ tt('Updated') }}: {{ displayTime(original.updatedTime) }}
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
import SnackBar from '@/components/desktop/SnackBar.vue';

import { ref, computed, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useUserStore } from '@/stores/user.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { useWishlistStore } from '@/stores/wishlist.ts';

import { BudgetPlanItem } from '@/models/budget_plan.ts';

import { parseDateTimeFromUnixTime } from '@/lib/datetime.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt, formatDateTimeToShortDateTime } = useI18n();

const userStore = useUserStore();
const accountsStore = useAccountsStore();
const wishlistStore = useWishlistStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

const showState = ref<boolean>(false);
const submitting = ref<boolean>(false);
const original = ref<BudgetPlanItem>(BudgetPlanItem.createNewWish());
const name = ref<string>('');
const amount = ref<number>(0);
const comment = ref<string>('');
const url = ref<string>('');
const groupId = ref<string>('');

let resolveFunc: ((wish: BudgetPlanItem) => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const isNew = computed<boolean>(() => !original.value.id);

// an older wish may still name an account, whose currency it is priced in
const currency = computed<string>(() => accountsStore.allAccountsMap[original.value.accountId]?.currency ?? userStore.currentUserDefaultCurrency);

const groupOptions = computed<{ id: string, name: string }[]>(() => [{ id: '', name: tt('Ungrouped') }].concat(wishlistStore.groups.map(group => ({ id: group.id, name: group.name }))));

const isInputValid = computed<boolean>(() => !!name.value.trim() && amount.value > 0);

function displayTime(unixTime: number): string {
    return formatDateTimeToShortDateTime(parseDateTimeFromUnixTime(unixTime));
}

function open(wish: BudgetPlanItem): Promise<BudgetPlanItem> {
    showState.value = true;
    submitting.value = false;
    original.value = wish;
    name.value = wish.name;
    amount.value = wish.amount;
    comment.value = wish.comment;
    url.value = wish.url;
    groupId.value = wish.wishGroupId;

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

    const wish = original.value.clone();
    wish.name = name.value.trim();
    wish.amount = amount.value;
    wish.comment = comment.value;
    // a link pasted without its scheme ("shop.com/sofa") is still meant as a web address
    const link = url.value.trim();
    wish.url = link && !/^[a-z][a-z0-9+.-]*:\/\//i.test(link) ? 'https://' + link : link;
    wish.wishGroupId = groupId.value;

    wishlistStore.saveWish({ wish: wish }).then(saved => {
        submitting.value = false;
        showState.value = false;
        resolveFunc?.(saved);
    }).catch(error => {
        submitting.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function cancel(): void {
    showState.value = false;
    rejectFunc?.();
}

defineExpose({
    open
});
</script>
