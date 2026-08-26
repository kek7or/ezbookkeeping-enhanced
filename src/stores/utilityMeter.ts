import { ref, computed } from 'vue';
import { defineStore } from 'pinia';

import type {
    UtilityMeterInfoResponse,
    UtilityMeterCreateRequest,
    UtilityMeterModifyRequest,
    UtilityTariffCreateRequest,
    UtilityTariffModifyRequest,
    UtilityReadingCreateRequest,
    UtilityReadingModifyRequest
} from '@/models/utility_meter.ts';

import logger from '@/lib/logger.ts';
import services from '@/lib/services.ts';

// rejectRequest turns whatever went wrong into the one shape the pages of this application show,
// keeping the server's own message when it sent one - "this meter has already been read on this
// day" says what to do about it, and "unable to save" does not
function rejectRequest(error: { processed?: boolean, response?: { data?: { errorMessage?: string } } }, message: string, reject: (reason?: unknown) => void): void {
    if (error.response && error.response.data && error.response.data.errorMessage) {
        reject({ error: error.response.data });
    } else if (!error.processed) {
        reject({ message: message });
    } else {
        reject(error);
    }
}

export const useUtilityMetersStore = defineStore('utilityMeters', () => {
    const allMeters = ref<UtilityMeterInfoResponse[]>([]);
    const metersStateInvalid = ref<boolean>(true);

    const allMetersMap = computed<Record<string, UtilityMeterInfoResponse>>(() => {
        const map: Record<string, UtilityMeterInfoResponse> = {};

        for (const meter of allMeters.value) {
            map[meter.id] = meter;
        }

        return map;
    });

    function updateMetersStateInvalid(invalidState: boolean): void {
        metersStateInvalid.value = invalidState;
    }

    function resetStore(): void {
        allMeters.value = [];
        metersStateInvalid.value = true;
    }

    // every meter comes back with what it has used, what that cost and where the year is heading,
    // all of it worked out by the server. Nothing here recalculates any of it, so that the numbers
    // on the page and the numbers in the tests are the same numbers.
    function loadAllMeters({ force }: { force: boolean }): Promise<UtilityMeterInfoResponse[]> {
        if (!force && !metersStateInvalid.value) {
            return Promise.resolve(allMeters.value);
        }

        return new Promise((resolve, reject) => {
            services.getAllUtilityMeters().then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to retrieve your meters' });
                    return;
                }

                allMeters.value = data.result;
                metersStateInvalid.value = false;

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to load utility meters', error);
                rejectRequest(error, 'Unable to retrieve your meters', reject);
            });
        });
    }

    // a reading, a price or a meter changed means every total of that meter changed with it, and
    // the totals are the server's to work out - so each of these ends by asking for the lot again
    // rather than patching a number here that was calculated there
    function reloadAfterChange<T>(result: T): Promise<T> {
        metersStateInvalid.value = true;

        return loadAllMeters({ force: true }).then(() => result).catch(() => result);
    }

    function addMeter(req: UtilityMeterCreateRequest): Promise<UtilityMeterInfoResponse> {
        return new Promise((resolve, reject) => {
            services.addUtilityMeter(req).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to add this meter' });
                    return;
                }

                reloadAfterChange(data.result).then(resolve);
            }).catch(error => {
                logger.error('failed to add utility meter', error);
                rejectRequest(error, 'Unable to add this meter', reject);
            });
        });
    }

    function modifyMeter(req: UtilityMeterModifyRequest): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.modifyUtilityMeter(req).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to save this meter' });
                    return;
                }

                reloadAfterChange(true).then(resolve);
            }).catch(error => {
                logger.error('failed to modify utility meter', error);
                rejectRequest(error, 'Unable to save this meter', reject);
            });
        });
    }

    function deleteMeter({ id }: { id: string }): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.deleteUtilityMeter({ id: id }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to delete this meter' });
                    return;
                }

                reloadAfterChange(true).then(resolve);
            }).catch(error => {
                logger.error('failed to delete utility meter', error);
                rejectRequest(error, 'Unable to delete this meter', reject);
            });
        });
    }

    function addTariff(req: UtilityTariffCreateRequest): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.addUtilityTariff(req).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to add this tariff' });
                    return;
                }

                reloadAfterChange(true).then(resolve);
            }).catch(error => {
                logger.error('failed to add utility tariff', error);
                rejectRequest(error, 'Unable to add this tariff', reject);
            });
        });
    }

    function modifyTariff(req: UtilityTariffModifyRequest): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.modifyUtilityTariff(req).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to save this tariff' });
                    return;
                }

                reloadAfterChange(true).then(resolve);
            }).catch(error => {
                logger.error('failed to modify utility tariff', error);
                rejectRequest(error, 'Unable to save this tariff', reject);
            });
        });
    }

    function deleteTariff({ id }: { id: string }): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.deleteUtilityTariff({ id: id }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to delete this tariff' });
                    return;
                }

                reloadAfterChange(true).then(resolve);
            }).catch(error => {
                logger.error('failed to delete utility tariff', error);
                rejectRequest(error, 'Unable to delete this tariff', reject);
            });
        });
    }

    function addReading(req: UtilityReadingCreateRequest): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.addUtilityReading(req).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to save this reading' });
                    return;
                }

                reloadAfterChange(true).then(resolve);
            }).catch(error => {
                logger.error('failed to add utility meter reading', error);
                rejectRequest(error, 'Unable to save this reading', reject);
            });
        });
    }

    function modifyReading(req: UtilityReadingModifyRequest): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.modifyUtilityReading(req).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to save this reading' });
                    return;
                }

                reloadAfterChange(true).then(resolve);
            }).catch(error => {
                logger.error('failed to modify utility meter reading', error);
                rejectRequest(error, 'Unable to save this reading', reject);
            });
        });
    }

    function deleteReading({ id }: { id: string }): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.deleteUtilityReading({ id: id }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to delete this reading' });
                    return;
                }

                reloadAfterChange(true).then(resolve);
            }).catch(error => {
                logger.error('failed to delete utility meter reading', error);
                rejectRequest(error, 'Unable to delete this reading', reject);
            });
        });
    }

    return {
        // states
        allMeters,
        metersStateInvalid,
        // computed states
        allMetersMap,
        // functions
        updateMetersStateInvalid,
        resetStore,
        loadAllMeters,
        addMeter,
        modifyMeter,
        deleteMeter,
        addTariff,
        modifyTariff,
        deleteTariff,
        addReading,
        modifyReading,
        deleteReading
    };
});
