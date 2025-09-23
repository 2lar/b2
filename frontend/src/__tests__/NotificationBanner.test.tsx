import { describe, it, expect, vi } from 'vitest';
import TestRenderer from 'react-test-renderer';
import NotificationBanner from '../common/components/NotificationBanner';
import styles from '../common/components/NotificationBanner.module.css';

describe('NotificationBanner', () => {
    it('renders the provided message', () => {
        const testRenderer = TestRenderer.create(
            <NotificationBanner message="Hello world" variant="success" />
        );

        const messageNode = testRenderer.root.findByProps({ className: styles.message });
        expect(messageNode.children).toEqual(['Hello world']);
    });

    it('calls the dismiss callback when the button is clicked', () => {
        const onDismiss = vi.fn();
        const testRenderer = TestRenderer.create(
            <NotificationBanner message="Closable" onDismiss={onDismiss} />
        );

        const button = testRenderer.root.findAllByType('button')[0];
        button.props.onClick();

        expect(onDismiss).toHaveBeenCalledTimes(1);
    });
});
