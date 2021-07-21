import React from "react";
import { withStyles } from "@material-ui/core";

const styles = {
    h1: {
        color: "#a4a4a4",
        margin: "5px 0px",
    },
    h2: {
        margin: "15px 0px",
    },
};

class ErrorBoundary extends React.Component {
    constructor(props) {
        super(props);
        this.state = { hasError: false, error: null, errorInfo: null };
    }

    static getDerivedStateFromError() {
        // 更新 state 使下一次繪製能夠顯示降級後的 UI
        return { hasError: true };
    }

    componentDidCatch(error, errorInfo) {
        this.setState({
            error: error,
            errorInfo: errorInfo,
        });
    }

    render() {
        const { classes } = this.props;
        if (this.state.hasError) {
            // 你可以自訂降級後的 UI 並繪製
            return (
                <>
                    <h1 className={classes.h1}>:(</h1>
                    <h2 className={classes.h2}>
                        頁面繪製出現錯誤，請嘗試重新整理此頁面。
                    </h2>
                    {this.state.error &&
                        this.state.errorInfo &&
                        this.state.errorInfo.componentStack && (
                            <details>
                                <summary>錯誤詳情</summary>
                                <pre>
                                    <code>{this.state.error.toString()}</code>
                                </pre>
                                <pre>
                                    <code>
                                        {this.state.errorInfo.componentStack}
                                    </code>
                                </pre>
                            </details>
                        )}
                </>
            );
        }

        return this.props.children;
    }
}

export default withStyles(styles)(ErrorBoundary);
