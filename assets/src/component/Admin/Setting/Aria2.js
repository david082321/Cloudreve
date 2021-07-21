import Button from "@material-ui/core/Button";
import FormControl from "@material-ui/core/FormControl";
import FormHelperText from "@material-ui/core/FormHelperText";
import Input from "@material-ui/core/Input";
import InputLabel from "@material-ui/core/InputLabel";
import Link from "@material-ui/core/Link";
import { makeStyles } from "@material-ui/core/styles";
import Typography from "@material-ui/core/Typography";
import Alert from "@material-ui/lab/Alert";
import React, { useCallback, useEffect, useState } from "react";
import { useDispatch } from "react-redux";
import { toggleSnackbar } from "../../../actions";
import API from "../../../middleware/Api";

const useStyles = makeStyles((theme) => ({
    root: {
        [theme.breakpoints.up("md")]: {
            marginLeft: 100,
        },
        marginBottom: 40,
    },
    form: {
        maxWidth: 400,
        marginTop: 20,
        marginBottom: 20,
    },
    formContainer: {
        [theme.breakpoints.up("md")]: {
            padding: "0px 24px 0 24px",
        },
    },
}));

export default function Aria2() {
    const classes = useStyles();
    const [loading, setLoading] = useState(false);
    const [options, setOptions] = useState({
        aria2_rpcurl: "",
        aria2_token: "",
        aria2_temp_path: "",
        aria2_options: "",
        aria2_interval: "0",
        aria2_call_timeout: "0",
    });

    const handleChange = (name) => (event) => {
        setOptions({
            ...options,
            [name]: event.target.value,
        });
    };

    const dispatch = useDispatch();
    const ToggleSnackbar = useCallback(
        (vertical, horizontal, msg, color) =>
            dispatch(toggleSnackbar(vertical, horizontal, msg, color)),
        [dispatch]
    );

    useEffect(() => {
        API.post("/admin/setting", {
            keys: Object.keys(options),
        })
            .then((response) => {
                setOptions(response.data);
            })
            .catch((error) => {
                ToggleSnackbar("top", "right", error.message, "error");
            });
        // eslint-disable-next-line
    }, []);

    const reload = () => {
        API.get("/admin/reload/aria2")
            // eslint-disable-next-line @typescript-eslint/no-empty-function
            .then(() => {})
            .catch((error) => {
                ToggleSnackbar("top", "right", error.message, "error");
            })
            // eslint-disable-next-line @typescript-eslint/no-empty-function
            .then(() => {});
    };

    const test = () => {
        setLoading(true);
        API.post("/admin/aria2/test", {
            server: options.aria2_rpcurl,
            token: options.aria2_token,
        })
            .then((response) => {
                ToggleSnackbar(
                    "top",
                    "right",
                    "連線成功，Aria2 版本為：" + response.data,
                    "success"
                );
            })
            .catch((error) => {
                ToggleSnackbar("top", "right", error.message, "error");
            })
            .then(() => {
                setLoading(false);
            });
    };

    const submit = (e) => {
        e.preventDefault();
        setLoading(true);
        const option = [];
        Object.keys(options).forEach((k) => {
            option.push({
                key: k,
                value: options[k],
            });
        });
        API.patch("/admin/setting", {
            options: option,
        })
            .then(() => {
                ToggleSnackbar("top", "right", "設定已更改", "success");
                reload();
            })
            .catch((error) => {
                ToggleSnackbar("top", "right", error.message, "error");
            })
            .then(() => {
                setLoading(false);
            });
    };

    return (
        <div>
            <form onSubmit={submit}>
                <div className={classes.root}>
                    <Typography variant="h6" gutterBottom>
                        Aria2
                    </Typography>

                    <div className={classes.formContainer}>
                        <div className={classes.form}>
                            <Alert severity="info" style={{ marginTop: 8 }}>
                                <Typography variant="body2">
                                    Cloudreve 的離線下載功能由{" "}
                                    <Link
                                        href={"https://aria2.github.io/"}
                                        target={"_blank"}
                                    >
                                        Aria2
                                    </Link>{" "}
                                    驅動。如需使用，請在同一裝置上以和執行
                                    Cloudreve 相同的使用者身份啟動 Aria2， 並在
                                    Aria2 的配置檔案中開啟 RPC
                                    服務。更多訊息及指引請參考文件的{" "}
                                    <Link
                                        href={
                                            "https://docs.cloudreve.org/use/aria2"
                                        }
                                        target={"_blank"}
                                    >
                                        離線下載
                                    </Link>{" "}
                                    章節。
                                </Typography>
                            </Alert>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    RPC 伺服器地址
                                </InputLabel>
                                <Input
                                    type={"url"}
                                    value={options.aria2_rpcurl}
                                    onChange={handleChange("aria2_rpcurl")}
                                />
                                <FormHelperText id="component-helper-text">
                                    包含埠的完整 RPC
                                    伺服器地址，例如：http://127.0.0.1:6800/，留空表示不啟用
                                    Aria2 服務
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    RPC Secret
                                </InputLabel>
                                <Input
                                    value={options.aria2_token}
                                    onChange={handleChange("aria2_token")}
                                />
                                <FormHelperText id="component-helper-text">
                                    RPC 授權令牌，與 Aria2
                                    配置檔案中保持一致，未設定請留空。
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    臨時下載目錄
                                </InputLabel>
                                <Input
                                    value={options.aria2_temp_path}
                                    onChange={handleChange("aria2_temp_path")}
                                />
                                <FormHelperText id="component-helper-text">
                                    離線下載臨時下載目錄的
                                    <strong>絕對路徑</strong>，Cloudreve
                                    行程需要此目錄的讀、寫、執行權限。
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    狀態重新整理間隔 (秒)
                                </InputLabel>
                                <Input
                                    type={"number"}
                                    inputProps={{
                                        step: 1,
                                        min: 1,
                                    }}
                                    required
                                    value={options.aria2_interval}
                                    onChange={handleChange("aria2_interval")}
                                />
                                <FormHelperText id="component-helper-text">
                                    Cloudreve 向 Aria2 請求重新整理任務狀態的間隔。
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    RPC 呼叫超時 (秒)
                                </InputLabel>
                                <Input
                                    type={"number"}
                                    inputProps={{
                                        step: 1,
                                        min: 1,
                                    }}
                                    required
                                    value={options.aria2_call_timeout}
                                    onChange={handleChange(
                                        "aria2_call_timeout"
                                    )}
                                />
                                <FormHelperText id="component-helper-text">
                                    呼叫 RPC 服務時最長等待時間
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    全域任務參數
                                </InputLabel>
                                <Input
                                    multiline
                                    required
                                    value={options.aria2_options}
                                    onChange={handleChange("aria2_options")}
                                />
                                <FormHelperText id="component-helper-text">
                                    建立下載任務時攜帶的額外設定參數，以 JSON
                                    編碼後的格式書寫，您可也可以將這些設定寫在
                                    Aria2 配置檔案裡，可用參數請查閱官方文件
                                </FormHelperText>
                            </FormControl>
                        </div>
                    </div>
                </div>

                <div className={classes.root}>
                    <Button
                        disabled={loading}
                        type={"submit"}
                        variant={"contained"}
                        color={"primary"}
                    >
                        儲存
                    </Button>
                    <Button
                        style={{ marginLeft: 8 }}
                        disabled={loading}
                        onClick={() => test()}
                        variant={"outlined"}
                        color={"secondary"}
                    >
                        測試連接
                    </Button>
                </div>
            </form>
        </div>
    );
}
