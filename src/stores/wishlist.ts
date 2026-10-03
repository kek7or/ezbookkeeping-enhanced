import { ref } from 'vue';
import { defineStore } from 'pinia';

import { type BudgetWishGroup, BudgetPlanItem } from '@/models/budget_plan.ts';

import services, { type ApiResponsePromise } from '@/lib/services.ts';
import logger from '@/lib/logger.ts';

// The wishlist page: every group and every wish, attached to a month or not. The budget page keeps
// its own copy of the wishes attached to the month it shows, and reads it back when it is opened.
export const useWishlistStore = defineStore('wishlist', () => {
    const groups = ref<BudgetWishGroup[]>([]);
    const wishes = ref<BudgetPlanItem[]>([]);

    // Every call here fails the same way, so the unwrapping is written once
    function call<T>(request: ApiResponsePromise<T>, failMessage: string): Promise<T> {
        return request.then(response => {
            const data = response.data;

            if (!data || !data.success || data.result === undefined || data.result === null) {
                throw new Error(failMessage);
            }

            return data.result;
        }).catch(error => {
            logger.error(failMessage, error);

            if (error.response && error.response.data && error.response.data.errorMessage) {
                return Promise.reject({ error: error.response.data });
            } else if (!error.processed) {
                return Promise.reject({ message: failMessage });
            }

            return Promise.reject(error);
        });
    }

    function loadWishlist(): Promise<void> {
        return call(services.getBudgetWishlist(), 'Unable to retrieve the wishlist').then(result => {
            groups.value = result.groups || [];
            wishes.value = BudgetPlanItem.ofMulti(result.wishes || []);
        });
    }

    function saveWish({ wish }: { wish: BudgetPlanItem }): Promise<BudgetPlanItem> {
        const isNew = !wish.id;
        const request = isNew ? services.addBudgetPlanWish(wish.toWishCreateRequest()) : services.modifyBudgetPlanWish(wish.toWishModifyRequest());

        return call(request, isNew ? 'Unable to add this wish' : 'Unable to save this wish').then(result => {
            const saved = BudgetPlanItem.of(result);
            const index = wishes.value.findIndex(existing => existing.id === saved.id);

            if (index >= 0) {
                wishes.value.splice(index, 1, saved);
            } else {
                wishes.value.push(saved);
            }

            return saved;
        });
    }

    function deleteWish({ wish }: { wish: BudgetPlanItem }): Promise<void> {
        return call(services.deleteBudgetPlanItem({ id: wish.id }), 'Unable to remove this wish').then(() => {
            wishes.value = wishes.value.filter(existing => existing.id !== wish.id);
        });
    }

    // year and month both zero takes the wish off any month
    function attachWish({ wish, year, month }: { wish: BudgetPlanItem, year: number, month: number }): Promise<void> {
        return call(services.attachBudgetPlanWish({ id: wish.id, year: year, month: month }), 'Unable to attach this to the month').then(() => {
            wish.year = year;
            wish.month = month;
        });
    }

    // Planning a wish is the decision to buy it: it leaves the wishlist and counts in the month
    function planWish({ wish, year, month }: { wish: BudgetPlanItem, year: number, month: number }): Promise<void> {
        return call(services.assignBudgetPlanWish({ id: wish.id, year: year, month: month }), 'Unable to put this into the month').then(() => {
            wishes.value = wishes.value.filter(existing => existing.id !== wish.id);
        });
    }

    // Grabbing takes a planned item back out of its month and onto the wishlist
    function grabPlannedItem({ item }: { item: BudgetPlanItem }): Promise<void> {
        return call(services.unassignBudgetPlanWish({ id: item.id }), 'Unable to return this to the wishlist').then(() => loadWishlist());
    }

    // Saves where every wish in the given lists now sits. Each list is one group, in order.
    function moveWishes(lists: { groupId: string, wishes: BudgetPlanItem[] }[]): Promise<void> {
        const newDisplayOrders = lists.flatMap(list => list.wishes.map((wish, index) => {
            wish.wishGroupId = list.groupId;
            wish.displayOrder = index + 1;

            return { id: wish.id, groupId: list.groupId || '0', displayOrder: index + 1 };
        }));

        if (!newDisplayOrders.length) {
            return Promise.resolve();
        }

        return call(services.moveBudgetPlanWishes({ newDisplayOrders: newDisplayOrders }), 'Unable to move these wishes').then(() => undefined);
    }

    function addGroup({ name }: { name: string }): Promise<void> {
        return call(services.addBudgetWishGroup({ name: name }), 'Unable to add this group').then(group => {
            groups.value.push(group);
        });
    }

    function renameGroup({ group, name }: { group: BudgetWishGroup, name: string }): Promise<void> {
        return call(services.modifyBudgetWishGroup({ id: group.id, name: name }), 'Unable to rename this group').then(() => {
            group.name = name;
        });
    }

    function moveGroups(): Promise<void> {
        const newDisplayOrders = groups.value.map((group, index) => {
            group.displayOrder = index + 1;
            return { id: group.id, displayOrder: index + 1 };
        });

        if (!newDisplayOrders.length) {
            return Promise.resolve();
        }

        return call(services.moveBudgetWishGroups({ newDisplayOrders: newDisplayOrders }), 'Unable to move these groups').then(() => undefined);
    }

    // What was in the group becomes ungrouped
    function deleteGroup({ group }: { group: BudgetWishGroup }): Promise<void> {
        return call(services.deleteBudgetWishGroup({ id: group.id }), 'Unable to delete this group').then(() => {
            groups.value = groups.value.filter(existing => existing.id !== group.id);

            for (const wish of wishes.value) {
                if (wish.wishGroupId === group.id) {
                    wish.wishGroupId = '';
                }
            }
        });
    }

    return {
        // states
        groups,
        wishes,
        // functions
        loadWishlist,
        saveWish,
        deleteWish,
        attachWish,
        planWish,
        grabPlannedItem,
        moveWishes,
        addGroup,
        renameGroup,
        moveGroups,
        deleteGroup
    };
});
