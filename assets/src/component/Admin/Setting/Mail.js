import Button from "@material-ui/core/Button";
import Dialog from "@material-ui/core/Dialog";
import DialogActions from "@material-ui/core/DialogActions";
import DialogContent from "@material-ui/core/DialogContent";
import DialogContentText from "@material-ui/core/DialogContentText";
import DialogTitle from "@material-ui/core/DialogTitle";
import FormControl from "@material-ui/core/FormControl";
import FormHelperText from "@material-ui/core/FormHelperText";
import Input from "@material-ui/core/Input";
import InputLabel from "@material-ui/core/InputLabel";
import { makeStyles } from "@material-ui/core/styles";
import TextField from "@material-ui/core/TextField";
import Typography from "@material-ui/core/Typography";
import React, { useCallback, useEffect, useState } from "react";
import { useDispatch } from "react-redux";
import { toggleSnackbar } from "../../../actions";
import API from "../../../middleware/Api";
import FormControlLabel from "@material-ui/core/FormControlLabel";
import Switch from "@material-ui/core/Switch";

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
    buttonMargin: {
        marginLeft: 8,
    },
}));

export default function Mail() {
    const classes = useStyles();
    const [loading, setLoading] = useState(false);
    const [test, setTest] = useState(false);
    const [tesInput, setTestInput] = useState("");
    const [options, setOptions] = useState({
        fromName: "",
        fromAdress: "",
        smtpHost: "",
        smtpPort: "",
        replyTo: "",
        smtpUser: "",
        smtpPass: "",
        smtpEncryption: "",
        mail_keepalive: "30",
        mail_activation_template: "",
        mail_reset_pwd_template: "",
    });

    const handleChange = (name) => (event) => {
        setOptions({
            ...options,
            [name]: event.target.value,
        });
    };

    const handleCheckChange = (name) => (event) => {
        let value = event.target.value;
        if (event.target.checked !== undefined) {
            value = event.target.checked ? "1" : "0";
        }
        setOptions({
            ...options,
            [name]: value,
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

    const sendTestMail = () => {
        setLoading(true);
        API.post("/admin/mailTest", {
            to: tesInput,
        })
            .then(() => {
                ToggleSnackbar("top", "right", "測試郵件已發送", "success");
            })
            .catch((error) => {
                ToggleSnackbar("top", "right", error.message, "error");
            })
            .then(() => {
                setLoading(false);
            });
    };

    const reload = () => {
        API.get("/admin/reload/email")
            // eslint-disable-next-line @typescript-eslint/no-empty-function
            .then(() => {})
            .catch((error) => {
                ToggleSnackbar("top", "right", error.message, "error");
            })
            // eslint-disable-next-line @typescript-eslint/no-empty-function
            .then(() => {});
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
            <Dialog
                open={test}
                onClose={() => setTest(false)}
                aria-labelledby="form-dialog-title"
            >
                <DialogTitle id="form-dialog-title">發件測試</DialogTitle>
                <DialogContent>
                    <DialogContentText>
                        <Typography>
                            發送測試郵件前，請先儲存已更改的郵件設定；
                        </Typography>
                        <Typography>
                            郵件發送結果不會立即回饋，如果您長時間未收到測試郵件，請檢查
                            Cloudreve 在終端輸出的錯誤日誌。
                        </Typography>
                    </DialogContentText>
                    <TextField
                        autoFocus
                        margin="dense"
                        id="name"
                        label="收件人地址"
                        value={tesInput}
                        onChange={(e) => setTestInput(e.target.value)}
                        type="email"
                        fullWidth
                    />
                </DialogContent>
                <DialogActions>
                    <Button onClick={() => setTest(false)} color="default">
                        取消
                    </Button>
                    <Button
                        onClick={() => sendTestMail()}
                        disabled={loading}
                        color="primary"
                    >
                        發送
                    </Button>
                </DialogActions>
            </Dialog>

            <form onSubmit={submit}>
                <div className={classes.root}>
                    <Typography variant="h6" gutterBottom>
                        發信
                    </Typography>

                    <div className={classes.formContainer}>
                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    發件人名
                                </InputLabel>
                                <Input
                                    value={options.fromName}
                                    onChange={handleChange("fromName")}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    郵件中展示的發件人姓名
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    發件人信箱
                                </InputLabel>
                                <Input
                                    type={"email"}
                                    required
                                    value={options.fromAdress}
                                    onChange={handleChange("fromAdress")}
                                />
                                <FormHelperText id="component-helper-text">
                                    發件信箱的地址
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    SMTP 伺服器
                                </InputLabel>
                                <Input
                                    value={options.smtpHost}
                                    onChange={handleChange("smtpHost")}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    發件伺服器地址，不含埠號
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    SMTP 埠
                                </InputLabel>
                                <Input
                                    inputProps={{ min: 1, step: 1 }}
                                    type={"number"}
                                    value={options.smtpPort}
                                    onChange={handleChange("smtpPort")}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    發件伺服器地址埠號
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    SMTP 使用者名稱
                                </InputLabel>
                                <Input
                                    value={options.smtpUser}
                                    onChange={handleChange("smtpUser")}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    發信信箱使用者名稱，一般與信箱地址相同
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    SMTP 密碼
                                </InputLabel>
                                <Input
                                    type={"password"}
                                    value={options.smtpPass}
                                    onChange={handleChange("smtpPass")}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    發信信箱密碼
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    回信信箱
                                </InputLabel>
                                <Input
                                    value={options.replyTo}
                                    onChange={handleChange("replyTo")}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    使用者回復系統發送的郵件時，用於接收回信的信箱
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <FormControlLabel
                                    control={
                                        <Switch
                                            checked={
                                                options.smtpEncryption === "1"
                                            }
                                            onChange={handleCheckChange(
                                                "smtpEncryption"
                                            )}
                                        />
                                    }
                                    label="強制使用 SSL 連接"
                                />
                                <FormHelperText id="component-helper-text">
                                    是否強制使用 SSL
                                    加密連接。如果無法發送郵件，可關閉此項，
                                    Cloudreve 會嘗試使用 STARTTLS
                                    並決定是否使用加密連接
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    SMTP 連接有效期 (秒)
                                </InputLabel>
                                <Input
                                    inputProps={{ min: 1, step: 1 }}
                                    type={"number"}
                                    value={options.mail_keepalive}
                                    onChange={handleChange("mail_keepalive")}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    有效期內建立的 SMTP
                                    連接會被新郵件發送請求復用
                                </FormHelperText>
                            </FormControl>
                        </div>
                    </div>
                </div>

                <div className={classes.root}>
                    <Typography variant="h6" gutterBottom>
                        郵件模板
                    </Typography>

                    <div className={classes.formContainer}>
                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    新使用者啟動
                                </InputLabel>
                                <Input
                                    value={options.mail_activation_template}
                                    onChange={handleChange(
                                        "mail_activation_template"
                                    )}
                                    multiline
                                    rowsMax="10"
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    新使用者註冊後啟動郵件的模板
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    重設密碼
                                </InputLabel>
                                <Input
                                    value={options.mail_reset_pwd_template}
                                    onChange={handleChange(
                                        "mail_reset_pwd_template"
                                    )}
                                    multiline
                                    rowsMax="10"
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    密碼重設郵件模板
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
                    {"   "}
                    <Button
                        className={classes.buttonMargin}
                        variant={"outlined"}
                        color={"primary"}
                        onClick={() => setTest(true)}
                    >
                        發送測試郵件
                    </Button>
                </div>
            </form>
        </div>
    );
}
