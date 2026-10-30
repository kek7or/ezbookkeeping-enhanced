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
                <wish-card :wish="element" :display-amount="displayAmount" :display-month="displayMonth" :category-name="categoryName"
                           @attach="emit('attach', $event)" @detach="emit('detach', $event)" @plan="emit('plan', $event)"
                           @edit="emit('edit', $event)" @remove="emit('remove', $event)"/>
            </template>
            <template #footer v-if="!list.wishes.length">
                <div class="text-caption text-medium-emphasis text-center py-4">{{ tt('Drop wishes here') }}</div>
            </template>
        </draggable-list>

        <!-- what was planned from this group stays here, out of the drag order, which is its month's -->
        <div class="wish-column-list pt-0" v-if="planned.length">
            <div class="text-caption text-medium-emphasis">{{ tt('Planned') }}</div>
            <wish-card :key="wish.id" :wish="wish" :display-amount="displayAmount" :display-month="displayMonth" :category-name="categoryName"
                       @unplan="emit('unplan', $event)" @edit="emit('edit', $event)"
                       v-for="wish in planned"/>
        </div>
    </v-card>
</template>

<script setup lang="ts">
import WishCard from './WishCard.vue';

import { useI18n } from '@/locales/helpers.ts';

import type { BudgetPlanItem } from '@/models/budget_plan.ts';

// one group's wishes in display order; dragging moves wishes between these arrays
export interface WishList {
    groupId: string;
    wishes: BudgetPlanItem[];
}

defineProps<{
    title: string;
    total: string;
    list: WishList;
    planned: BudgetPlanItem[];
    displayAmount: (wish: BudgetPlanItem) => string;
    displayMonth: (wish: BudgetPlanItem) => string;
    categoryName: (categoryId: string) => string;
}>();

const emit = defineEmits<{
    (e: 'update', wishes: BudgetPlanItem[]): void;
    (e: 'moved', list: WishList): void;
    (e: 'attach', wish: BudgetPlanItem): void;
    (e: 'detach', wish: BudgetPlanItem): void;
    (e: 'plan', wish: BudgetPlanItem): void;
    (e: 'unplan', wish: BudgetPlanItem): void;
    (e: 'edit', wish: BudgetPlanItem): void;
    (e: 'remove', wish: BudgetPlanItem): void;
}>();

const { tt } = useI18n();
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
</style>
