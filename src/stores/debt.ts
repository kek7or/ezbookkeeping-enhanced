import { ref, computed } from 'vue';
import { defineStore } from 'pinia';

import type {
    DebtPersonInfoResponse,
    DebtEntryInfoResponse,
    DebtEntryCreateBatchRequest,
    DebtEntryCreateManualRequest
} from '@/models/debt.ts';

import { KnownFileType } from '@/core/file.ts';

import logger from '@/lib/logger.ts';
import services from '@/lib/services.ts';

// DebtReceiptFile is a receipt as it comes off the server: what is in it, and what the server called
// it. The name is the server's to give, because the document is the server's to write.
export interface DebtReceiptFile {
    readonly content: Blob;
    readonly fileName: string;
}

// getAttachmentFileName reads the name the server gave the file it is sending back.
//
// A response that names nothing, or names something with a path in it, is treated as having named
// nothing at all - the caller then falls back to a name of its own, rather than letting a header
// decide where on the disk a file is written.
function getAttachmentFileName(contentDisposition: string): string {
    const match = /filename=("?)([^";]+)\1/.exec(contentDisposition);
    const fileName = (match?.[2] ?? '').trim();

    if (!fileName || fileName.indexOf('/') >= 0 || fileName.indexOf('\\') >= 0 || fileName.indexOf('..') >= 0) {
        return '';
    }

    return fileName;
}

// getResponseHeader reads a header off a response as the one string it is meant to be. Axios types a
// header as anything a header could ever hold - one value, several, or none at all
function getResponseHeader(header: unknown): string {
    if (Array.isArray(header)) {
        return (header[0] ?? '').toString();
    }

    if (header === null || header === undefined) {
        return '';
    }

    return header.toString();
}

// readErrorBlob reads back the message of a refused request that was asked for as a blob.
//
// The blob is asked what it is rather than the headers being read again, because that is the one
// answer that is certainly about this body. Anything that is not text yields nothing - the caller
// then says only that it could not be done, which is still true and is better than showing the
// reader a spreadsheet's worth of bytes.
function readErrorBlob(error: { response?: { data?: unknown } }): Promise<string> {
    const data = error.response?.data;

    if (!(data instanceof Blob) || data.type.indexOf('text/') !== 0) {
        return Promise.resolve('');
    }

    return data.text().then(text => text.trim()).catch(() => '');
}

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

    // exportReceipt fetches what a person still owes as a spreadsheet to be handed to them.
    //
    // The sheet is written by the server rather than here, because a receipt is a document and not a
    // view: it has to be the same file whoever asks for it, and it has to hold dates and amounts a
    // spreadsheet can still add up rather than the words this page happens to print them as.
    function exportReceipt({ personId }: { personId: string }): Promise<DebtReceiptFile> {
        return new Promise((resolve, reject) => {
            services.exportDebtReceipt({ personId: personId }).then(response => {
                if (!response || !response.data) {
                    reject({ message: 'Unable to make a receipt for this person' });
                    return;
                }

                const contentType = getResponseHeader(response.headers['content-type']);

                if (!KnownFileType.XLSX.isSameType(contentType)) {
                    reject({ message: 'Unable to make a receipt for this person' });
                    return;
                }

                resolve({
                    content: new Blob([response.data], { type: contentType }),
                    fileName: getAttachmentFileName(getResponseHeader(response.headers['content-disposition']))
                });
            }).catch(error => {
                logger.error('failed to export the receipt of a debt person', error);

                if (error.processed) {
                    reject(error);
                    return;
                }

                // the request asked for a blob, so a refusal arrives as one too and has to be read
                // back into the words it was written as before it can be shown
                readErrorBlob(error).then(message => {
                    reject(message ? { message: 'error.' + message } : { message: 'Unable to make a receipt for this person' });
                });
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
        reopenEntries,
        exportReceipt
    };
});
