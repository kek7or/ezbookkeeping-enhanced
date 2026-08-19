export const TRANSACTION_MIN_AMOUNT: number = -999999999999999; // -9,999,999,999,999.99
export const TRANSACTION_MAX_AMOUNT: number = 999999999999999; // 9,999,999,999,999.99
export const TRANSACTION_MAX_PICTURE_COUNT: number = 10;
export const TRANSACTION_MAX_COMMENT_LENGTH: number = 255;
export const TRANSACTION_COMMENT_HINT_MIN_LENGTH: number = TRANSACTION_MAX_COMMENT_LENGTH - 100;
// matches the column a receipt name is stored in, so a name that fits here is never refused by the server
export const TRANSACTION_RECEIPT_MAX_MERCHANT_NAME_LENGTH: number = 255;
