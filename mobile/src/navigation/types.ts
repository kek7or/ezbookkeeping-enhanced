import type { NativeStackScreenProps } from '@react-navigation/native-stack';

export type RootStackParamList = {
    Home: undefined;
    /**
     * `photoId` prefills the form from a recognised receipt and links the photo
     * to the resulting transaction. `transactionId` reopens a queued row the
     * server rejected, so it can be corrected and retried.
     */
    AddTransaction: { photoId?: number; transactionId?: number } | undefined;
    Camera: undefined;
    Review: undefined;
};

export type ScreenProps<T extends keyof RootStackParamList> = NativeStackScreenProps<RootStackParamList, T>;
