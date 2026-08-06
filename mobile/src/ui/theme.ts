import { StyleSheet } from 'react-native';

export const colors = {
    background: '#f5f6f8',
    surface: '#ffffff',
    border: '#dfe3e8',
    text: '#1c2024',
    textMuted: '#6b7280',
    primary: '#1a6dd4',
    primaryText: '#ffffff',
    danger: '#c8352c',
    success: '#1f8a4c',
    warning: '#b8730d',
    expense: '#c8352c',
    income: '#1f8a4c'
};

export const spacing = {
    xs: 4,
    sm: 8,
    md: 12,
    lg: 16,
    xl: 24
};

export const styles = StyleSheet.create({
    screen: {
        flex: 1,
        backgroundColor: colors.background
    },
    content: {
        padding: spacing.lg,
        gap: spacing.md
    },
    card: {
        backgroundColor: colors.surface,
        borderRadius: 10,
        borderWidth: 1,
        borderColor: colors.border,
        padding: spacing.lg,
        gap: spacing.sm
    },
    label: {
        fontSize: 13,
        fontWeight: '600',
        color: colors.textMuted,
        textTransform: 'uppercase',
        letterSpacing: 0.4
    },
    input: {
        borderWidth: 1,
        borderColor: colors.border,
        borderRadius: 8,
        paddingHorizontal: spacing.md,
        paddingVertical: spacing.md,
        fontSize: 16,
        color: colors.text,
        backgroundColor: colors.surface
    },
    title: {
        fontSize: 20,
        fontWeight: '700',
        color: colors.text
    },
    subtitle: {
        fontSize: 14,
        color: colors.textMuted
    },
    body: {
        fontSize: 15,
        color: colors.text
    },
    button: {
        backgroundColor: colors.primary,
        borderRadius: 8,
        paddingVertical: spacing.md,
        paddingHorizontal: spacing.lg,
        alignItems: 'center',
        justifyContent: 'center'
    },
    buttonText: {
        color: colors.primaryText,
        fontSize: 16,
        fontWeight: '600'
    },
    buttonSecondary: {
        backgroundColor: colors.surface,
        borderWidth: 1,
        borderColor: colors.border
    },
    buttonSecondaryText: {
        color: colors.text
    },
    buttonDisabled: {
        opacity: 0.5
    },
    row: {
        flexDirection: 'row',
        alignItems: 'center',
        gap: spacing.sm
    },
    errorText: {
        color: colors.danger,
        fontSize: 14
    }
});
