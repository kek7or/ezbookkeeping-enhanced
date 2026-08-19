import { ref, computed } from 'vue';
import { defineStore } from 'pinia';

import type {
    DebtPersonInfoResponse,
    DebtEntryInfoResponse,
    DebtEntryCreateBatchRequest,
    DebtEntryCreateManualRequest
} from '@/models/debt.ts';

import logger from '@/lib/logger.ts';
import services from '@/lib/services.ts';

export const useDebtsStore = defineStore('debts', () => {
    const allPeople = ref<DebtPersonInfoResponse[]>([]);
    const peopleStateInvalid = ref<boolean>(true);

    // the entries of one person at a time, because the page shows one person at a time and what is
    // owed changes under every settlement
    const currentPersonId = ref<string>('');
    const currentEntries = ref<DebtEntryInfoResponse[]>([]);

    const allPeopleMap = computed<Record<string, DebtPersonInfoResponse>>(() => {
        const map: Record<string, DebtPersonInfoResponse> = {};

        for (const person of allPeople.value) {
            map[person.id] = person;
        }

        return map;
    });

    const openEntries = computed<DebtEntryInfoResponse[]>(() => currentEntries.value.filter(entry => !entry.settled));
    const settledEntries = computed<DebtEntryInfoResponse[]>(() => currentEntries.value.filter(entry => entry.settled));

    function resetDebts(): void {
        allPeople.value = [];
        peopleStateInvalid.value = true;
        currentPersonId.value = '';
        currentEntries.value = [];
    }

    function updatePeopleStateInvalid(invalidState: boolean): void {
        peopleStateInvalid.value = invalidState;
    }

    function loadAllPeople({ force }: { force: boolean }): Promise<DebtPersonInfoResponse[]> {
        if (!force && !peopleStateInvalid.value) {
            return Promise.resolve(allPeople.value);
        }

        return new Promise((resolve, reject) => {
            services.getAllDebtPeople().then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to retrieve the people who owe you' });
                    return;
                }

                allPeople.value = data.result;
                peopleStateInvalid.value = false;

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to load debt people', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to retrieve the people who owe you' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function addPerson({ name }: { name: string }): Promise<DebtPersonInfoResponse> {
        return new Promise((resolve, reject) => {
            services.addDebtPerson({ name: name }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to add this person' });
                    return;
                }

                allPeople.value.push(data.result);

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to add debt person', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to add this person' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function renamePerson({ id, name }: { id: string, name: string }): Promise<DebtPersonInfoResponse> {
        return new Promise((resolve, reject) => {
            services.modifyDebtPerson({ id: id, name: name }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to rename this person' });
                    return;
                }

                peopleStateInvalid.value = true;

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to rename debt person', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to rename this person' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function deletePerson({ id }: { id: string }): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.deleteDebtPerson({ id: id }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to delete this person' });
                    return;
                }

                allPeople.value = allPeople.value.filter(person => person.id !== id);

                if (currentPersonId.value === id) {
                    currentPersonId.value = '';
                    currentEntries.value = [];
                }

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to delete debt person', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to delete this person' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function loadEntries({ personId, includeSettled }: { personId: string, includeSettled: boolean }): Promise<DebtEntryInfoResponse[]> {
        return new Promise((resolve, reject) => {
            services.getDebtEntries({ personId: personId, includeSettled: includeSettled }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to retrieve what this person owes' });
                    return;
                }

                currentPersonId.value = personId;
                currentEntries.value = data.result;

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to load debt entries', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to retrieve what this person owes' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function loadEntriesOfTransaction({ transactionId }: { transactionId: string }): Promise<DebtEntryInfoResponse[]> {
        return new Promise((resolve, reject) => {
            services.getDebtEntriesByTransaction({ transactionId: transactionId }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to retrieve who owes for this transaction' });
                    return;
                }

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to load debt entries of transaction', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to retrieve who owes for this transaction' });
                } else {
                    reject(error);
                }
            });
        });
    }

    // attachEntries says that somebody owes for transactions or receipt positions. What they owe in
    // total has changed, so the list of people is marked stale rather than patched here.
    function attachEntries(req: DebtEntryCreateBatchRequest): Promise<DebtEntryInfoResponse[]> {
        return new Promise((resolve, reject) => {
            services.addDebtEntries(req).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to attach this to the person' });
                    return;
                }

                peopleStateInvalid.value = true;

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to attach debt entries', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to attach this to the person' });
                } else {
                    reject(error);
                }
            });
        });
    }

    // addManualEntry records a debt that never passed through the ledger - cash handed over, a bill
    // paid from somewhere this program does not keep
    function addManualEntry(req: DebtEntryCreateManualRequest): Promise<DebtEntryInfoResponse> {
        return new Promise((resolve, reject) => {
            services.addManualDebtEntry(req).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to record this loan' });
                    return;
                }

                peopleStateInvalid.value = true;

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to add manual debt entry', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to record this loan' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function modifyEntry({ id, amount, description }: { id: string, amount: number, description?: string }): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.modifyDebtEntry({ id: id, amount: amount, description: description }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to change what is owed' });
                    return;
                }

                peopleStateInvalid.value = true;

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to modify debt entry', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to change what is owed' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function deleteEntries({ ids }: { ids: string[] }): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.deleteDebtEntries({ ids: ids }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to detach this from the person' });
                    return;
                }

                peopleStateInvalid.value = true;
                currentEntries.value = currentEntries.value.filter(entry => ids.indexOf(entry.id) < 0);

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to delete debt entries', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to detach this from the person' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function settleEntries({ ids, settlementTransactionId }: { ids: string[], settlementTransactionId: string }): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.settleDebtEntries({ ids: ids, settlementTransactionId: settlementTransactionId }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to record this repayment' });
                    return;
                }

                peopleStateInvalid.value = true;

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to settle debt entries', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to record this repayment' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function reopenEntries({ ids }: { ids: string[] }): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.reopenDebtEntries({ ids: ids }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to put this back on the bill' });
                    return;
                }

                peopleStateInvalid.value = true;

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to reopen debt entries', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to put this back on the bill' });
                } else {
                    reject(error);
                }
            });
        });
    }

    return {
        // states
        allPeople,
        peopleStateInvalid,
        currentPersonId,
        currentEntries,
        // computed states
        allPeopleMap,
        openEntries,
        settledEntries,
        // functions
        resetDebts,
        updatePeopleStateInvalid,
        loadAllPeople,
        addPerson,
        renamePerson,
        deletePerson,
        loadEntries,
        loadEntriesOfTransaction,
        attachEntries,
        addManualEntry,
        modifyEntry,
        deleteEntries,
        settleEntries,
        reopenEntries
    };
});
