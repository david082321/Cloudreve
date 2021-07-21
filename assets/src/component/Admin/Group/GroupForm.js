import Button from "@material-ui/core/Button";
import Collapse from "@material-ui/core/Collapse";
import FormControl from "@material-ui/core/FormControl";
import FormControlLabel from "@material-ui/core/FormControlLabel";
import FormHelperText from "@material-ui/core/FormHelperText";
import Input from "@material-ui/core/Input";
import InputLabel from "@material-ui/core/InputLabel";
import MenuItem from "@material-ui/core/MenuItem";
import Select from "@material-ui/core/Select";
import { makeStyles } from "@material-ui/core/styles";
import Switch from "@material-ui/core/Switch";
import Typography from "@material-ui/core/Typography";
import React, { useCallback, useEffect, useState } from "react";
import { useDispatch } from "react-redux";
import { useHistory } from "react-router";
import { toggleSnackbar } from "../../../actions";
import API from "../../../middleware/Api";
import SizeInput from "../Common/SizeInput";

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

// function getStyles(name, personName, theme) {
//     return {
//         fontWeight:
//             personName.indexOf(name) === -1
//                 ? theme.typography.fontWeightRegular
//                 : theme.typography.fontWeightMedium
//     };
// }

export default function GroupForm(props) {
    const classes = useStyles();
    const [loading, setLoading] = useState(false);
    const [group, setGroup] = useState(
        props.group
            ? props.group
            : {
                  ID: 0,
                  Name: "",
                  MaxStorage: "1073741824", // 轉換類型
                  ShareEnabled: "true", // 轉換類型
                  WebDAVEnabled: "true", // 轉換類型
                  SpeedLimit: "0", // 轉換類型
                  PolicyList: 1, // 轉換類型,至少選擇一個
                  OptionsSerialized: {
                      // 批次轉換類型
                      share_download: "true",
                      aria2_options: "{}", // json decode
                      compress_size: "0",
                      decompress_size: "0",
                  },
              }
    );
    const [policies, setPolicies] = useState({});

    const history = useHistory();

    const dispatch = useDispatch();
    const ToggleSnackbar = useCallback(
        (vertical, horizontal, msg, color) =>
            dispatch(toggleSnackbar(vertical, horizontal, msg, color)),
        [dispatch]
    );

    useEffect(() => {
        API.post("/admin/policy/list", {
            page: 1,
            page_size: 10000,
            order_by: "id asc",
            conditions: {},
        })
            .then((response) => {
                const res = {};
                response.data.items.forEach((v) => {
                    res[v.ID] = v.Name;
                });
                setPolicies(res);
            })
            .catch((error) => {
                ToggleSnackbar("top", "right", error.message, "error");
            });
    }, []);

    const handleChange = (name) => (event) => {
        setGroup({
            ...group,
            [name]: event.target.value,
        });
    };

    const handleCheckChange = (name) => (event) => {
        const value = event.target.checked ? "true" : "false";
        setGroup({
            ...group,
            [name]: value,
        });
    };

    const handleOptionCheckChange = (name) => (event) => {
        const value = event.target.checked ? "true" : "false";
        setGroup({
            ...group,
            OptionsSerialized: {
                ...group.OptionsSerialized,
                [name]: value,
            },
        });
    };

    const handleOptionChange = (name) => (event) => {
        setGroup({
            ...group,
            OptionsSerialized: {
                ...group.OptionsSerialized,
                [name]: event.target.value,
            },
        });
    };

    const submit = (e) => {
        e.preventDefault();
        const groupCopy = {
            ...group,
            OptionsSerialized: { ...group.OptionsSerialized },
        };

        // 布林值轉換
        ["ShareEnabled", "WebDAVEnabled"].forEach((v) => {
            groupCopy[v] = groupCopy[v] === "true";
        });
        [
            "archive_download",
            "archive_task",
            "one_time_download",
            "share_download",
            "aria2",
        ].forEach((v) => {
            if (groupCopy.OptionsSerialized[v] !== undefined) {
                groupCopy.OptionsSerialized[v] =
                    groupCopy.OptionsSerialized[v] === "true";
            }
        });

        // 整型轉換
        ["MaxStorage", "SpeedLimit"].forEach((v) => {
            groupCopy[v] = parseInt(groupCopy[v]);
        });
        ["compress_size", "decompress_size"].forEach((v) => {
            if (groupCopy.OptionsSerialized[v] !== undefined) {
                groupCopy.OptionsSerialized[v] = parseInt(
                    groupCopy.OptionsSerialized[v]
                );
            }
        });
        groupCopy.PolicyList = [parseInt(groupCopy.PolicyList)];
        // JSON轉換
        try {
            groupCopy.OptionsSerialized.aria2_options = JSON.parse(
                groupCopy.OptionsSerialized.aria2_options
            );
        } catch (e) {
            ToggleSnackbar("top", "right", "Aria2 設定項格式錯誤", "warning");
            return;
        }

        setLoading(true);
        API.post("/admin/group", {
            group: groupCopy,
        })
            .then(() => {
                history.push("/admin/group");
                ToggleSnackbar(
                    "top",
                    "right",
                    "使用者群組已" + (props.group ? "儲存" : "添加"),
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

    return (
        <div>
            <form onSubmit={submit}>
                <div className={classes.root}>
                    <Typography variant="h6" gutterBottom>
                        {group.ID === 0 && "建立使用者群組"}
                        {group.ID !== 0 && "編輯 " + group.Name}
                    </Typography>

                    <div className={classes.formContainer}>
                        {group.ID !== 3 && (
                            <>
                                <div className={classes.form}>
                                    <FormControl fullWidth>
                                        <InputLabel htmlFor="component-helper">
                                            使用者群組名
                                        </InputLabel>
                                        <Input
                                            value={group.Name}
                                            onChange={handleChange("Name")}
                                            required
                                        />
                                        <FormHelperText id="component-helper-text">
                                            使用者群組的名稱
                                        </FormHelperText>
                                    </FormControl>
                                </div>

                                <div className={classes.form}>
                                    <FormControl fullWidth>
                                        <InputLabel htmlFor="component-helper">
                                            儲存策略
                                        </InputLabel>
                                        <Select
                                            labelId="demo-mutiple-chip-label"
                                            id="demo-mutiple-chip"
                                            value={group.PolicyList}
                                            onChange={handleChange(
                                                "PolicyList"
                                            )}
                                            input={
                                                <Input id="select-multiple-chip" />
                                            }
                                        >
                                            {Object.keys(policies).map(
                                                (pid) => (
                                                    <MenuItem
                                                        key={pid}
                                                        value={pid}
                                                    >
                                                        {policies[pid]}
                                                    </MenuItem>
                                                )
                                            )}
                                        </Select>
                                        <FormHelperText id="component-helper-text">
                                            指定使用者群組的儲存策略。
                                        </FormHelperText>
                                    </FormControl>
                                </div>

                                <div className={classes.form}>
                                    <FormControl fullWidth>
                                        <SizeInput
                                            value={group.MaxStorage}
                                            onChange={handleChange(
                                                "MaxStorage"
                                            )}
                                            min={0}
                                            max={9223372036854775807}
                                            label={"初始容量"}
                                            required
                                        />
                                    </FormControl>
                                    <FormHelperText id="component-helper-text">
                                        使用者群組下的使用者初始可用最大容量
                                    </FormHelperText>
                                </div>
                            </>
                        )}

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <SizeInput
                                    value={group.SpeedLimit}
                                    onChange={handleChange("SpeedLimit")}
                                    min={0}
                                    max={9223372036854775807}
                                    label={"下載限速"}
                                    suffix={"/s"}
                                    required
                                />
                            </FormControl>
                            <FormHelperText id="component-helper-text">
                                填寫為 0 表示不限制。開啟限制後，
                                此使用者群組下的使用者下載所有支援限速的儲存策略下的文件時，下載最大速度會被限制。
                            </FormHelperText>
                        </div>

                        {group.ID !== 3 && (
                            <div className={classes.form}>
                                <FormControl fullWidth>
                                    <FormControlLabel
                                        control={
                                            <Switch
                                                checked={
                                                    group.ShareEnabled ===
                                                    "true"
                                                }
                                                onChange={handleCheckChange(
                                                    "ShareEnabled"
                                                )}
                                            />
                                        }
                                        label="允許建立分享"
                                    />
                                    <FormHelperText id="component-helper-text">
                                        關閉後，使用者無法建立分享連結
                                    </FormHelperText>
                                </FormControl>
                            </div>
                        )}

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <FormControlLabel
                                    control={
                                        <Switch
                                            checked={
                                                group.OptionsSerialized
                                                    .share_download === "true"
                                            }
                                            onChange={handleOptionCheckChange(
                                                "share_download"
                                            )}
                                        />
                                    }
                                    label="允許下載分享"
                                />
                                <FormHelperText id="component-helper-text">
                                    關閉後，使用者無法下載別人建立的文件分享
                                </FormHelperText>
                            </FormControl>
                        </div>

                        {group.ID !== 3 && (
                            <div className={classes.form}>
                                <FormControl fullWidth>
                                    <FormControlLabel
                                        control={
                                            <Switch
                                                checked={
                                                    group.WebDAVEnabled ===
                                                    "true"
                                                }
                                                onChange={handleCheckChange(
                                                    "WebDAVEnabled"
                                                )}
                                            />
                                        }
                                        label="WebDAV"
                                    />
                                    <FormHelperText id="component-helper-text">
                                        關閉後，使用者無法透過 WebDAV
                                        協議連接至網路硬碟
                                    </FormHelperText>
                                </FormControl>
                            </div>
                        )}

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <FormControlLabel
                                    control={
                                        <Switch
                                            checked={
                                                group.OptionsSerialized
                                                    .one_time_download ===
                                                "true"
                                            }
                                            onChange={handleOptionCheckChange(
                                                "one_time_download"
                                            )}
                                        />
                                    }
                                    label="禁止多次下載請求"
                                />
                                <FormHelperText id="component-helper-text">
                                    只針對本機儲存策略有效。開啟後，使用者無法使用多執行緒下載工具。
                                </FormHelperText>
                            </FormControl>
                        </div>

                        {group.ID !== 3 && (
                            <div className={classes.form}>
                                <FormControl fullWidth>
                                    <FormControlLabel
                                        control={
                                            <Switch
                                                checked={
                                                    group.OptionsSerialized
                                                        .aria2 === "true"
                                                }
                                                onChange={handleOptionCheckChange(
                                                    "aria2"
                                                )}
                                            />
                                        }
                                        label="離線下載"
                                    />
                                    <FormHelperText id="component-helper-text">
                                        是否允許使用者建立離線下載任務
                                    </FormHelperText>
                                </FormControl>
                            </div>
                        )}

                        <Collapse in={group.OptionsSerialized.aria2 === "true"}>
                            <div className={classes.form}>
                                <FormControl fullWidth>
                                    <InputLabel htmlFor="component-helper">
                                        Aria2 任務參數
                                    </InputLabel>
                                    <Input
                                        multiline
                                        value={
                                            group.OptionsSerialized
                                                .aria2_options
                                        }
                                        onChange={handleOptionChange(
                                            "aria2_options"
                                        )}
                                    />
                                    <FormHelperText id="component-helper-text">
                                        此使用者群組建立離線下載任務時額外攜帶的參數，以
                                        JSON
                                        編碼後的格式書寫，您可也可以將這些設定寫在
                                        Aria2 配置檔案裡，可用參數請查閱官方文件
                                    </FormHelperText>
                                </FormControl>
                            </div>
                        </Collapse>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <FormControlLabel
                                    control={
                                        <Switch
                                            checked={
                                                group.OptionsSerialized
                                                    .archive_download === "true"
                                            }
                                            onChange={handleOptionCheckChange(
                                                "archive_download"
                                            )}
                                        />
                                    }
                                    label="打包下載"
                                />
                                <FormHelperText id="component-helper-text">
                                    是否允許使用者多選文件打包下載
                                </FormHelperText>
                            </FormControl>
                        </div>

                        {group.ID !== 3 && (
                            <div className={classes.form}>
                                <FormControl fullWidth>
                                    <FormControlLabel
                                        control={
                                            <Switch
                                                checked={
                                                    group.OptionsSerialized
                                                        .archive_task === "true"
                                                }
                                                onChange={handleOptionCheckChange(
                                                    "archive_task"
                                                )}
                                            />
                                        }
                                        label="壓縮/解壓縮 任務"
                                    />
                                    <FormHelperText id="component-helper-text">
                                        是否使用者建立 壓縮/解壓縮 任務
                                    </FormHelperText>
                                </FormControl>
                            </div>
                        )}

                        <Collapse
                            in={group.OptionsSerialized.archive_task === "true"}
                        >
                            <div className={classes.form}>
                                <FormControl fullWidth>
                                    <SizeInput
                                        value={
                                            group.OptionsSerialized
                                                .compress_size
                                        }
                                        onChange={handleOptionChange(
                                            "compress_size"
                                        )}
                                        min={0}
                                        max={9223372036854775807}
                                        label={"待壓縮文件最大大小"}
                                    />
                                </FormControl>
                                <FormHelperText id="component-helper-text">
                                    使用者可建立的壓縮任務的文件最大總大小，填寫為
                                    0 表示不限制
                                </FormHelperText>
                            </div>

                            <div className={classes.form}>
                                <FormControl fullWidth>
                                    <SizeInput
                                        value={
                                            group.OptionsSerialized
                                                .decompress_size
                                        }
                                        onChange={handleOptionChange(
                                            "decompress_size"
                                        )}
                                        min={0}
                                        max={9223372036854775807}
                                        label={"待解壓文件最大大小"}
                                    />
                                </FormControl>
                                <FormHelperText id="component-helper-text">
                                    使用者可建立的解壓縮任務的文件最大總大小，填寫為
                                    0 表示不限制
                                </FormHelperText>
                            </div>
                        </Collapse>
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
                </div>
            </form>
        </div>
    );
}
