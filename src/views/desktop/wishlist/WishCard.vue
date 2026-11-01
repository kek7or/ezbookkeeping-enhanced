<template>
    <v-card class="wish-card" variant="flat" :class="{ 'wish-card-planned': planned }"
            @mouseenter="hovered = true" @mouseleave="hovered = false">
        <div class="d-flex align-start">
            <span class="text-body-1 font-weight-medium wish-card-name" @click="copy(wish.name, 'name')">
                {{ wish.name }}
                <v-tooltip activator="parent" location="top">{{ copiedKey === 'name' ? tt('Copied') : tt('Copy Name') }}</v-tooltip>
            </span>
            <v-spacer/>
            <span class="text-body-1 text-no-wrap ms-2">{{ displayAmount(wish) }}</span>
        </div>
        <div class="text-caption text-medium-emphasis d-flex align-center" v-if="wish.categoryId">
            <v-icon size="14" class="me-1" :icon="mdiShapeOutline"/>{{ categoryName(wish.categoryId) }}
        </div>
        <!-- planned before a category was required: counted in the month but shown nowhere there -->
        <div class="text-caption text-warning d-flex align-center" v-else-if="planned">
            <v-icon size="14" class="me-1" :icon="mdiAlertOutline"/>{{ tt('No category, so it is not shown in the budget. Edit to choose one.') }}
        </div>
        <div class="text-caption text-medium-emphasis wish-card-description" v-if="wish.comment">
            <template :key="index" v-for="(part, index) in splitLinks(wish.comment)">
                <a :href="part.text" target="_blank" rel="noopener noreferrer" v-if="part.link" @mousedown.stop>{{ part.text }}</a>
                <template v-else>{{ part.text }}</template>
            </template>
        </div>
        <!-- the month gets a row of its own, so the hover buttons never crowd it out -->
        <div class="mt-1" v-if="planned || wish.year">
            <!-- a planned wish counts in its month; closing the chip takes it back out of the month -->
            <v-chip size="x-small" color="success" variant="tonal" closable
                    :prepend-icon="mdiCalendarCheck"
                    v-if="planned" @click:close="emit('unplan', wish)">
                {{ tt('Planned') }}: {{ displayMonth(wish) }}
                <v-tooltip activator="parent">{{ tt('Counted in that month\'s budget. Close to take it back out of the month.') }}</v-tooltip>
            </v-chip>
            <v-chip size="x-small" color="primary" variant="tonal" closable
                    v-else @click:close="emit('detach', wish)">{{ displayMonth(wish) }}</v-chip>
        </div>
        <div class="d-flex align-center mt-1 wish-card-footer">
            <template v-if="wish.url">
                <a class="text-caption" :href="wish.url" target="_blank" rel="noopener noreferrer" @mousedown.stop>{{ tt('Web Link') }}</a>
                <v-btn density="compact" size="x-small" variant="text" :icon="true" @mousedown.stop @click="copy(wish.url, 'url')">
                    <v-icon size="14" :icon="copiedKey === 'url' ? mdiCheck : mdiContentCopy"/>
                    <v-tooltip activator="parent">{{ copiedKey === 'url' ? tt('Copied') : tt('Copy Link') }}</v-tooltip>
                </v-btn>
            </template>
            <v-spacer/>
            <div class="wish-card-buttons" v-if="hovered">
                <template v-if="!planned">
                    <v-btn density="compact" size="small" variant="text" :icon="true" @click="emit('attach', wish)">
                        <v-icon size="18" :icon="mdiCalendarHeart"/>
                        <v-tooltip activator="parent">{{ tt('Attach to Month') }}</v-tooltip>
                    </v-btn>
                    <!-- a span keeps the tooltip working on the disabled button -->
                    <span>
                        <v-btn density="compact" size="small" variant="text" color="primary" :icon="true"
                               :disabled="!wish.categoryId" @click="emit('plan', wish)">
                            <v-icon size="18" :icon="mdiCalendarCheck"/>
                        </v-btn>
                        <v-tooltip activator="parent">{{ wish.categoryId ? tt('Plan for Month') : tt('Choose a category to plan this for a month') }}</v-tooltip>
                    </span>
                </template>
                <v-btn density="compact" size="small" variant="text" color="primary" :icon="true" v-else @click="emit('buy', wish)">
                    <v-icon size="18" :icon="mdiCartCheck"/>
                    <v-tooltip activator="parent">{{ tt('Bought: Add Transaction') }}</v-tooltip>
                </v-btn>
                <v-btn density="compact" size="small" variant="text" :icon="true" @click="emit('edit', wish)">
                    <v-icon size="18" :icon="mdiPencilOutline"/>
                    <v-tooltip activator="parent">{{ tt('Edit') }}</v-tooltip>
                </v-btn>
                <!-- something planned goes back to the wishlist first; only a wish is deleted -->
                <v-btn density="compact" size="small" variant="text" :icon="true" v-if="!planned" @click="emit('remove', wish)">
                    <v-icon size="18" :icon="mdiDeleteOutline"/>
                    <v-tooltip activator="parent">{{ tt('Delete') }}</v-tooltip>
                </v-btn>
            </div>
        </div>
    </v-card>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import type { BudgetPlanItem } from '@/models/budget_plan.ts';

import { copyTextToClipboard } from '@/lib/ui/common.ts';

import { mdiPencilOutline, mdiDeleteOutline, mdiCalendarHeart, mdiCalendarCheck, mdiContentCopy, mdiCheck, mdiShapeOutline, mdiAlertOutline, mdiCartCheck } from '@mdi/js';

const props = defineProps<{
    wish: BudgetPlanItem;
    displayAmount: (wish: BudgetPlanItem) => string;
    displayMonth: (wish: BudgetPlanItem) => string;
    categoryName: (categoryId: string) => string;
}>();

const emit = defineEmits<{
    (e: 'attach', wish: BudgetPlanItem): void;
    (e: 'detach', wish: BudgetPlanItem): void;
    (e: 'plan', wish: BudgetPlanItem): void;
    (e: 'unplan', wish: BudgetPlanItem): void;
    (e: 'buy', wish: BudgetPlanItem): void;
    (e: 'edit', wish: BudgetPlanItem): void;
    (e: 'remove', wish: BudgetPlanItem): void;
}>();

const { tt } = useI18n();

const hovered = ref<boolean>(false);
// which copy was just made, "name" or "url", to say so for a moment
const copiedKey = ref<string>('');

const planned = computed<boolean>(() => !props.wish.wished);

function copy(text: string, key: string): void {
    copyTextToClipboard(text);
    copiedKey.value = key;
    setTimeout(() => {
        if (copiedKey.value === key) {
            copiedKey.value = '';
        }
    }, 1500);
}

// a description often carries the shop link, which is worth one click
function splitLinks(text: string): { text: string, link: boolean }[] {
    return text.split(/(https?:\/\/\S+)/).filter(part => part).map(part => ({ text: part, link: /^https?:\/\//.test(part) }));
}
</script>

<style scoped>
.wish-card {
    padding: 10px 12px 6px;
    cursor: grab;
}

.wish-card-planned {
    cursor: default;
    opacity: 0.85;
}

.wish-card-name {
    cursor: copy;
    word-break: break-word;
}

.wish-card-description {
    white-space: pre-line;
    word-break: break-word;
    display: -webkit-box;
    -webkit-line-clamp: 3;
    line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow: hidden;
}

.wish-card-footer {
    min-height: 28px;
}

.wish-card-buttons {
    display: inline-flex;
}
</style>
