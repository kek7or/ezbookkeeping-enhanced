import React from 'react';
import { ActivityIndicator, View } from 'react-native';
import { NavigationContainer } from '@react-navigation/native';
import { createNativeStackNavigator } from '@react-navigation/native-stack';
import { SafeAreaProvider } from 'react-native-safe-area-context';
import { StatusBar } from 'expo-status-bar';

import { AddTransactionScreen } from './src/screens/AddTransactionScreen';
import { AppProvider, useApp } from './src/state/AppContext';
import { CameraScreen } from './src/screens/CameraScreen';
import { HomeScreen } from './src/screens/HomeScreen';
import { LoginScreen } from './src/screens/LoginScreen';
import { ReviewScreen } from './src/screens/ReviewScreen';
import { colors } from './src/ui/theme';

import type { RootStackParamList } from './src/navigation/types';

const Stack = createNativeStackNavigator<RootStackParamList>();

function RootNavigator(): React.ReactElement {
    const { ready, session } = useApp();

    if (!ready) {
        return (
            <View
                style={{
                    flex: 1,
                    justifyContent: 'center',
                    alignItems: 'center',
                    backgroundColor: colors.background
                }}
            >
                <ActivityIndicator color={colors.primary} size="large" />
            </View>
        );
    }

    if (!session) {
        return <LoginScreen />;
    }

    return (
        <Stack.Navigator
            screenOptions={{
                headerStyle: { backgroundColor: colors.surface },
                headerTintColor: colors.text,
                contentStyle: { backgroundColor: colors.background }
            }}
        >
            <Stack.Screen name="Home" component={HomeScreen} options={{ title: 'ezbookkeeping' }} />
            <Stack.Screen
                name="AddTransaction"
                component={AddTransactionScreen}
                options={{ title: 'New transaction' }}
            />
            <Stack.Screen name="Camera" component={CameraScreen} options={{ title: 'Receipt' }} />
            <Stack.Screen name="Review" component={ReviewScreen} options={{ title: 'Review receipts' }} />
        </Stack.Navigator>
    );
}

export default function App(): React.ReactElement {
    return (
        <SafeAreaProvider>
            <AppProvider>
                <NavigationContainer>
                    <StatusBar style="dark" />
                    <RootNavigator />
                </NavigationContainer>
            </AppProvider>
        </SafeAreaProvider>
    );
}
