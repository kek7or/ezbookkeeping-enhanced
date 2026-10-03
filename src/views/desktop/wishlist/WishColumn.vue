<template>
    <v-card class="wish-column" variant="tonal">
        <div class="wish-column-header d-flex align-center">
            <slot name="handle"></slot>
            <span class="text-subtitle-1 font-weight-medium text-truncate">{{ title }}</span>
            <v-spacer/>
            <span class="text-body-2 font-weight-medium text-no-wrap me-1">{{ total }}</span>
            <slot name="actions"></slot>
        </div>

        <draggable-list class="wish-column-list" item-key="id" ghost-class="dragging-item"
                        group="wishes" :model-value="list.wishes"
                        @update:model-value="emit('update', $event)" @change="emit('moved', list)">
            <template #item="{ element }">
                <v-card class="wish-card" variant="flat"
                        @mouseenter="hoveredWishId = element.id" @mouseleave="hoveredWishId = ''">
                    <div class="d-flex align-start">
                        <span class="text-body-1 font-weight-medium wish-card-name">{{ element.name }}</span>
                        <v-spacer/>
                        <span class="text-body-1 text-no-wrap ms-2">{{ displayAmount(element) }}</span>
                    </div>
                    <div class="text-caption text-medium-emphasis wish-card-description" v-if="element.comment">
                        <template :key="index" v-for="(part, index) in splitLinks(element.comment)">
                            <a :href="part.text" target="_blank" rel="noopener noreferrer" v-if="part.link" @mousedown.stop>{{ part.text }}</a>
                            <template v-else>{{ part.text }}</template>
                        </template>
                    </div>
                    <div class="d-flex align-center mt-1 wish-card-footer">
                        <v-chip size="x-small" color="primary" variant="tonal" closable
                                v-if="element.year" @click:close="emit('detach', element)">{{ displayMonth(element) }}</v-chip>
                        <v-spacer/>
                        <div class="wish-card-buttons" :class="{ 'wish-card-buttons-shown': hoveredWishId === element.id }">
                            <v-btn density="compact" size="small" variant="text" :icon="true" @click="emit('attach', element)">
                                <v-icon size="18" :icon="mdiCalendarHeart"/>
                                <v-tooltip activator="parent">{{ tt('Attach to Month') }}</v-tooltip>
                            </v-btn>
                            <v-btn density="compact" size="small" variant="text" color="primary" :icon="true" @click="emit('plan', element)">
                                <v-icon size="18" :icon="mdiCalendarCheck"/>
                                <v-tooltip activator="parent">{{ tt('Plan for Month') }}</v-tooltip>
                            </v-btn>
                            <v-btn density="compact" size="small" variant="text" :icon="true" @click="emit('edit', element)">
                                <v-icon size="18" :icon="mdiPencilOutline"/>
                                <v-tooltip activator="parent">{{ tt('Edit') }}</v-tooltip>
                            </v-btn>
                            <v-btn density="compact" size="small" variant="text" :icon="true" @click="emit('remove', element)">
                                <v-icon size="18" :icon="mdiDeleteOutline"/>
                                <v-tooltip activator="parent">{{ tt('Delete') }}</v-tooltip>
                            </v-btn>
                        </div>
                    </div>
                </v-card>
            </template>
            <template #footer v-if="!list.wishes.length">
                <div class="text-caption text-medium-emphasis text-center py-4">{{ tt('Drop wishes here') }}</div>
            </template>
        </draggable-list>
    </v-card>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import type { BudgetPlanItem } from '@/models/budget_plan.ts';

import { mdiPencilOutline, mdiDeleteOutline, mdiCalendarHeart, mdiCalendarCheck } from '@mdi/js';

// one group's wishes in display order; dragging moves wishes between these arrays
export interface WishList {
    groupId: string;
    wishes: BudgetPlanItem[];
}

defineProps<{
    title: string;
    total: string;
    list: WishList;
    displayAmount: (wish: BudgetPlanItem) => string;
    displayMonth: (wish: BudgetPlanItem) => string;
}>();

const emit = defineEmits<{
    (e: 'update', wishes: BudgetPlanItem[]): void;
    (e: 'moved', list: WishList): void;
    (e: 'attach', wish: BudgetPlanItem): void;
    (e: 'detach', wish: BudgetPlanItem): void;
    (e: 'plan', wish: BudgetPlanItem): void;
    (e: 'edit', wish: BudgetPlanItem): void;
    (e: 'remove', wish: BudgetPlanItem): void;
}>();

const { tt } = useI18n();

const hoveredWishId = ref<string>('');

// a description often carries the shop link, which is worth one click
function splitLinks(text: string): { text: string, link: boolean }[] {
    return text.split(/(https?:\/\/\S+)/).filter(part => part).map(part => ({ text: part, link: /^https?:\/\//.test(part) }));
}
</script>

<style scoped>
.wish-column {
    width: 300px;
    flex: 0 0 300px;
    align-self: flex-start;
}

.wish-column-header {
    padding: 12px 8px 4px 12px;
    min-height: 48px;
}

.wish-column-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 8px;
    min-height: 64px;
}

.wish-card {
    padding: 10px 12px 6px;
    cursor: grab;
}

.wish-card-name {
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

/* the buttons are always laid out and only shown on hover, so the card does not jump */
.wish-card-buttons {
    display: inline-flex;
    visibility: hidden;
}

.wish-card-buttons-shown {
    visibility: visible;
}
</style>
